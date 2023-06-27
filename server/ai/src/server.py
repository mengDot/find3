import os
import time
import base58
import logging

from expiringdict import ExpiringDict


# create logger with 'spam_application'
logger = logging.getLogger('server')
logger.setLevel(logging.DEBUG)
fh = logging.FileHandler('server.log')
fh.setLevel(logging.DEBUG)
ch = logging.StreamHandler()
ch.setLevel(logging.DEBUG)
formatter = logging.Formatter(
    '%(asctime)s - [%(name)s/%(funcName)s] - %(levelname)s - %(message)s')
fh.setFormatter(formatter)
ch.setFormatter(formatter)
logger.addHandler(fh)
logger.addHandler(ch)


from flask import Flask, request, jsonify
app = Flask(__name__)


from learn import AI
from plot_locations import plot_data
ai_cache = ExpiringDict(max_len=100000, max_age_seconds=60)


def to_base58(family):
    return base58.b58encode(family.encode('utf-8')).decode('utf-8')

@app.route('/plot', methods=['POST'])
def plotdata():
    t = time.time()

    payload = request.get_json()
    if 'url' not in payload:
        return jsonify({'success': False, 'message': 'must provide callback url'})
    if 'data_folder' not in payload:
        return jsonify({'success': False, 'message': 'must provide data folder'})

    try:
        os.makedirs(payload['data_folder'])
    except:
        pass
    plot_data(payload['url'],payload['data_folder'])
    return jsonify({'success': True, 'message': 'generated data'})


@app.route('/classify', methods=['POST'])
def classify():
    t = time.time()

    payload = request.get_json()
    if payload is None:
        return jsonify({'success': False, 'message': 'must provide sensor data'})

    if 'sensor_data' not in payload:
        return jsonify({'success': False, 'message': 'must provide sensor data'})

    data_folder = '.'
    if 'data_folder' in payload:
        data_folder = payload['data_folder']

    fname = os.path.join(data_folder, to_base58(
        payload['sensor_data']['f']) + ".find3.ai")

    ai = ai_cache.get(payload['sensor_data']['f'])
    if ai == None:
        ai = AI(to_base58(payload['sensor_data']['f']), data_folder)
        logger.debug("loading {}".format(fname))
        try:
            ai.load(fname)
        except FileNotFoundError:
            return jsonify({"success": False, "message": "could not find '{p}'".format(p=fname)})
        ai_cache[payload['sensor_data']['f']] = ai

    classified = ai.classify(payload['sensor_data'])

    logger.debug("classifed for {} {:d} ms".format(
        payload['sensor_data']['f'], int(1000 * (t - time.time()))))
    return jsonify({"success": True, "message": "data analyzed", 'analysis': classified})


@app.route('/learn', methods=['POST'])
def learn():
    payload = request.get_json()
    if payload is None:
        return jsonify({'success': False, 'message': 'must provide sensor data'})
    if 'family' not in payload:
        return jsonify({'success': False, 'message': 'must provide family'})
    if 'csv_file' not in payload:
        return jsonify({'success': False, 'message': 'must provide CSV file'})
    data_folder = '.'
    if 'data_folder' in payload:
        data_folder = payload['data_folder']
    else:
        logger.debug("could not find data_folder in payload")

    logger.debug(data_folder)

    ai = AI(to_base58(payload['family']), data_folder)
    fname = os.path.join(data_folder, payload['csv_file'])
    try:
        ai.learn(fname)
    except FileNotFoundError:
        return jsonify({"success": False, "message": "could not find '{}'".format(fname)})

    print(payload['family'])
    ai.save(os.path.join(data_folder, to_base58(
        payload['family']) + ".find3.ai"))
    ai_cache[payload['family']] = ai
    return jsonify({"success": True, "message": "calibrated data"})


if __name__ == "__main__":
    app.run(host='0.0.0.0', port=8002)
