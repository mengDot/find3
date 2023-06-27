package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/pkg/errors"
	cache "github.com/robfig/go-cache"
	"github.com/schollz/find3/server/main/src/database"
	"github.com/schollz/find3/server/main/src/learning/nb1"
	"github.com/schollz/find3/server/main/src/models"
	"github.com/schollz/find3/server/main/src/utils"
)

// AIPort designates the port for the AI processing
var AIPort = "8002"
var DataFolder = "."

var (
	httpClient *http.Client
	routeCache *cache.Cache
)

const (
	MaxIdleConnections int = 20
	RequestTimeout     int = 300
)

// init HTTPClient
func init() {
	httpClient = createHTTPClient()
	routeCache = cache.New(5*time.Minute, 10*time.Minute)
}

// createHTTPClient for connection re-use
func createHTTPClient() *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: MaxIdleConnections,
		},
		Timeout: time.Duration(RequestTimeout) * time.Second,
	}

	return client
}

type AnalysisResponse struct {
	Data    models.LocationAnalysis `json:"analysis"`
	Message string                  `json:"message"`
	Success bool                    `json:"success"`
}

func AnalyzeSensorData(s models.SensorData) (aidata models.LocationAnalysis, err error) {
	startAnalyze := time.Now()

	aidata.Guesses = []models.LocationPrediction{}
	aidata.LocationNames = make(map[string]string)

	type a struct {
		aidata models.LocationAnalysis
		err    error
	}
	aChan := make(chan a)
	go func(aChan chan a) {
		// inquire the AI
		aiTime := time.Now()
		var target AnalysisResponse
		type ClassifyPayload struct {
			Sensor     models.SensorData `json:"sensor_data"`
			DataFolder string            `json:"data_folder"`
		}
		var p2 ClassifyPayload
		p2.Sensor = s
		p2.DataFolder = DataFolder
		url := "http://localhost:" + AIPort + "/classify"
		bPayload, err := json.Marshal(p2)
		if err != nil {
			err = errors.Wrap(err, "problem marshaling data")
			aChan <- a{err: err}
			return
		}
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(bPayload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			err = errors.Wrap(err, "problem posting payload")
			aChan <- a{err: err}
			return
		}
		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&target)
		if err != nil {
			err = errors.Wrap(err, "problem decoding response")
			aChan <- a{err: err}
			return
		}
		if !target.Success {
			err = errors.New("unable to analyze: " + target.Message)
			aChan <- a{err: err}
			return
		}
		if len(target.Data.Predictions) == 0 {
			err = errors.New("problem analyzing: no predictions")
			aChan <- a{err: err}
			return
		}
		logger.Log.Debugf("[%s] python classified %s", s.Family, time.Since(aiTime))
		aChan <- a{err: err, aidata: target.Data}
	}(aChan)

	type b struct {
		pl  nb1.PairList
		err error
	}
	bChan := make(chan b)
	go func(bChan chan b) {
		// do naive bayes1 learning
		nb1Time := time.Now()
		nb := nb1.New()
		pl, err := nb.Classify(s)
		logger.Log.Debugf("[%s] nb1 classified %s", s.Family, time.Since(nb1Time))
		bChan <- b{pl: pl, err: err}
	}(bChan)

	// type c struct {
	// 	pl  nb2.PairList
	// 	err error
	// }
	// cChan := make(chan c)
	// go func(cChan chan c) {
	// 	// do naive bayes2 learning
	// 	nb2Time := time.Now()
	// 	nbLearned2 := nb2.New()
	// 	pl, err := nbLearned2.Classify(s)
	// 	logger.Log.Debugf("[%s] nb2 classified %s", s.Family, time.Since(nb2Time))
	// 	cChan <- c{pl: pl, err: err}
	// }(cChan)

	aResult := <-aChan
	if aResult.err != nil || len(aResult.aidata.Predictions) == 0 {
		err = errors.Wrap(aResult.err, "problem with machine learnaing")
		logger.Log.Error(aResult.err)
		return
	}
	aidata = aResult.aidata

	reverseLocationNames := make(map[string]string)
	for key, value := range aidata.LocationNames {
		reverseLocationNames[value] = key
	}

	// process nb1
	bResult := <-bChan
	if bResult.err == nil {
		pl := bResult.pl
		algPrediction := models.AlgorithmPrediction{Name: "Extended Naive Bayes1"}
		algPrediction.Locations = make([]string, len(pl))
		algPrediction.Probabilities = make([]float64, len(pl))
		for i := range pl {
			algPrediction.Locations[i] = reverseLocationNames[pl[i].Key]
			algPrediction.Probabilities[i] = float64(int(pl[i].Value*100)) / 100
		}
		aidata.Predictions = append(aidata.Predictions, algPrediction)
	} else {
		logger.Log.Warnf("[%s] nb1 classify: %s", s.Family, bResult.err.Error())
	}

	// // process nb2
	// cResult := <-cChan
	// if cResult.err == nil {
	// 	pl2 := cResult.pl
	// 	algPrediction := models.AlgorithmPrediction{Name: "Extended Naive Bayes2"}
	// 	algPrediction.Locations = make([]string, len(pl2))
	// 	algPrediction.Probabilities = make([]float64, len(pl2))
	// 	for i := range pl2 {
	// 		algPrediction.Locations[i] = reverseLocationNames[pl2[i].Key]
	// 		algPrediction.Probabilities[i] = float64(int(pl2[i].Value*100)) / 100
	// 	}
	// 	aidata.Predictions = append(aidata.Predictions, algPrediction)
	// } else {
	// 	logger.Log.Warnf("[%s] nb2 classify: %s", s.Family, cResult.err.Error())
	// }

	d, err := database.Open(s.Family)
	if err != nil {
		return
	}
	var algorithmEfficacy map[string]map[string]models.BinaryStats
	d.Get("AlgorithmEfficacy", &algorithmEfficacy)
	d.Close()
	aidata.Guesses = determineBestGuess(aidata, algorithmEfficacy)

	if aidata.IsUnknown {
		aidata.Guesses = []models.LocationPrediction{
			{
				Location:    "?",
				Probability: 1,
			},
		}
	}

	// add prediction to the database
	// adding predictions uses up a lot of space
	go func() {
		d, err := database.Open(s.Family)
		if err != nil {
			return
		}
		defer d.Close()
		errInsert := d.AddPrediction(s.Timestamp, aidata.Guesses)
		if errInsert != nil {
			logger.Log.Errorf("[%s] problem inserting: %s", s.Family, errInsert.Error())
		}
	}()

	logger.Log.Debugf("[%s] analyzed in %s", s.Family, time.Since(startAnalyze))
	return
}

func determineBestGuess(aidata models.LocationAnalysis, algorithmEfficacy map[string]map[string]models.BinaryStats) (b []models.LocationPrediction) {
	// determine consensus
	locationScores := make(map[string]float64)
	for _, prediction := range aidata.Predictions {
		if len(prediction.Locations) == 0 {
			continue
		}
		for i := range prediction.Locations {
			guessedLocation := aidata.LocationNames[prediction.Locations[i]]
			if prediction.Probabilities[i] <= 0 {
				continue
			}
			if len(guessedLocation) == 0 {
				continue
			}
			efficacy := prediction.Probabilities[i] * algorithmEfficacy[prediction.Name][guessedLocation].Informedness
			if _, ok := locationScores[guessedLocation]; !ok {
				locationScores[guessedLocation] = float64(0)
			}
			if efficacy > 0 {
				locationScores[guessedLocation] += efficacy
			}
		}
	}

	total := float64(0)
	for location := range locationScores {
		total += locationScores[location]
	}

	pl := make(PairList, len(locationScores))
	i := 0
	for k, v := range locationScores {
		pl[i] = Pair{k, v / total}
		i++
	}
	sort.Sort(sort.Reverse(pl))

	b = make([]models.LocationPrediction, len(locationScores))
	for i := range pl {
		b[i].Location = pl[i].Key
		b[i].Probability = float64(int(pl[i].Value*100000)) / 100000
	}

	if len(locationScores) == 1 {
		b[0].Probability = 1
	}

	return b
}

type Pair struct {
	Key   string
	Value float64
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func GetByLocation(family string, minutesAgoInt int, showRandomized bool, activeMinsThreshold int, minScanners int, minProbability float64, deviceCounts map[string]int) (byLocations []models.ByLocation, err error) {
	millisecondsAgo := int64(minutesAgoInt * 60 * 1000)

	d, err := database.Open(family, true)
	if err != nil {
		return
	}
	defer d.Close()

	startTime := time.Now()
	sensors, err := d.GetSensorFromGreaterTime(millisecondsAgo)
	logger.Log.Debugf("[%s] got sensor from greater time %s", family, time.Since(startTime))

	startTime = time.Now()
	preAnalyzed := make(map[int64][]models.LocationPrediction)
	devicesToCheckMap := make(map[string]struct{})
	for _, sensor := range sensors {
		a, errGet := d.GetPrediction(sensor.Timestamp)
		if errGet != nil {
			continue
		}
		preAnalyzed[sensor.Timestamp] = a
		devicesToCheckMap[sensor.Device] = struct{}{}
	}
	logger.Log.Debugf("[%s] got predictions in map %s", family, time.Since(startTime))

	// get list of devices I care about
	devicesToCheck := make([]string, len(devicesToCheckMap))
	i := 0
	for device := range devicesToCheckMap {
		devicesToCheck[i] = device
		i++
	}
	logger.Log.Debugf("[%s] found %d devices to check", family, len(devicesToCheck))

	startTime = time.Now()
	if len(deviceCounts) == 0 {
		deviceCounts, err = d.GetDeviceCountsFromDevices(devicesToCheck)
		if err != nil {
			err = errors.Wrap(err, "could not get devices")
			return
		}
	}
	logger.Log.Debugf("[%s] got device counts %s", family, time.Since(startTime))

	startTime = time.Now()
	deviceFirstTime, err := d.GetDeviceFirstTimeFromDevices(devicesToCheck)
	if err != nil {
		err = errors.Wrap(err, "problem getting device first time")
		return
	}
	logger.Log.Debugf("[%s] got device first-time %s", family, time.Since(startTime))

	var rollingData models.ReverseRollingData
	errGotRollingData := d.Get("ReverseRollingData", &rollingData)

	d.Close()

	locations := make(map[string][]models.ByLocationDevice)
	for _, s := range sensors {
		isRandomized := utils.IsMacRandomized(s.Device)
		if !showRandomized && isRandomized {
			continue
		}
		if _, ok := deviceCounts[s.Device]; !ok {
			// logger.Log.Warnf("missing device counts for %s", s.Device)
			continue
		}
		if _, ok := deviceFirstTime[s.Device]; !ok {
			// logger.Log.Warnf("missing deviceFirstTime for %s", s.Device)
			continue
		}
		if errGotRollingData == nil {
			if int(deviceCounts[s.Device])*int(rollingData.TimeBlock.Seconds())/60 < activeMinsThreshold {
				continue
			}
		}

		var a []models.LocationPrediction
		if _, ok := preAnalyzed[s.Timestamp]; ok {
			a = preAnalyzed[s.Timestamp]
		} else {
			var aidata models.LocationAnalysis
			aidata, err = AnalyzeSensorData(s)
			if err != nil {
				return
			}
			a = aidata.Guesses
		}

		// filter on probability
		if a[0].Probability < minProbability {
			continue
		}

		if _, ok := locations[a[0].Location]; !ok {
			locations[a[0].Location] = []models.ByLocationDevice{}
		}
		numScanners := 0
		for sensorType := range s.Sensors {
			numScanners += len(s.Sensors[sensorType])
		}
		if numScanners < minScanners {
			continue
		}

		dL := models.ByLocationDevice{
			Device:      s.Device,
			Timestamp:   time.Unix(0, s.Timestamp*1000000).UTC(),
			Probability: a[0].Probability,
			Randomized:  isRandomized,
			NumScanners: numScanners,
			FirstSeen:   deviceFirstTime[s.Device],
		}
		if errGotRollingData == nil {
			dL.ActiveMins = int(deviceCounts[s.Device]) * int(rollingData.TimeBlock.Seconds()) / 60
		} else {
			dL.ActiveMins = int(deviceCounts[s.Device]*30) / 60
		}
		vendor, vendorErr := utils.GetVendorFromOUI(s.Device)
		if vendorErr == nil {
			dL.Vendor = vendor
		}
		locations[a[0].Location] = append(locations[a[0].Location], dL)
	}

	byLocations = make([]models.ByLocation, len(locations))
	i = 0
	gpsData, _ := GetGPSData(family)
	for location := range locations {
		byLocations[i].GPS = gpsData[location].GPS
		byLocations[i].Location = location
		byLocations[i].Devices = locations[location]
		byLocations[i].Total = len(locations[location])
		i++
	}
	return
}
