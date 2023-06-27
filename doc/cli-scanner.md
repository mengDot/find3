# Command-line scanner

## Introduction

The command-line scanner provides a means for your laptop your computer to monitor the address and signal of nearby WiFi and bluetooth devices (*active scanning*). Also, if equipped with a monitor-mode enabled WiFI card, you can use the scanner to intercept probe requests to do *passive scanning*.

## Install

There are a couple of ways to install. I recommend downloading the [latest release](https://github.com/schollz/find3-cli-scanner/releases/latest) as that is the easiest way.

### Install natively

First make sure you have [downloaded Go](https://golang.org/dl/).

Then, install the dependencies. 

*Ubuntu Linux*

```
$ sudo apt-get install wireless-tools net-tools libpcap-dev bluetooth
```

*Fedora Linux*

```
$ sudo dnf install golang wireless-tools net-tools libpcap-devel bluez bluez-tools
```

*OS X*

```
$ brew install libpcap
```

Now download and install the scanner with *go get*:

```
$ go get -u -v github.com/schollz/find3-cli-scanner
```

If you are on Linux, then you should move it to a path that is available with sudo:

```
$ sudo mv $GOPATH/bin/find3-cli-scanner /usr/local/bin
```

That's it! See below for usage.

### Install with Docker

Install Docker:

```
$ curl -sSL https://get.docker.com | sh
```

If *not* using a Raspberry Pi, fetch the latest image.

```
$ docker pull schollz/find3-cli-scanner
```

If you are using a Raspberry Pi (`armf` arch), you need to build the image yourself, although I suggest Raspberry Pi's just built natively above.

```
$ wget https://raw.githubusercontent.com/schollz/find3-cli-scanner/master/Dockerfile
$ docker build -t schollz/find3-cli-scanner .
```

Now you can start the scanning image in the background.

```
$ docker run --net="host" --privileged --name scanner -d -i -t schollz/find3-cli-scanner
```

To use the scanner, your syntax will be

```
$ docker exec scanner sh -c "find3-cli-scanner -device DEVICE -family FAMILY -wifi -bluetooth -forever"
```

Be sure to use your own device/family name. Use `-help` to see which flags are available.

You can start/stop the image using

```
$ docker start scanning
$ docker stop scanning
```

> Note, you can jump inside the image and play if you are curious of trying new things.
```
$ docker run --net="host" --privileged --name scanning -i -t scanner /bin/bash
```
> 


## Usage

### Active scanning 

In *active scanning* the scanner will report the classified location of the device that is doing the scanning.

To use the `find3-cli-scanner` you must include the name of your interface (typically something like `wlan0`) with `-i`. You can determine the name using `ifconfig` or similar command.

You must also include a **family name** with `-family` and a **device name** specified with `-device`. This will help organize your data among the server, so choose them to be unique. You can have multiple devices in the same family.

The default server is https://cloud.internalpositioning.com which you can specify with `-server`. 

I suggest using a scantime of about 10 seconds, which you can specify with `-scantime 10`. If you want bluetooth to be scanned as well, just add `-bluetooth`.

To keep the scanner running, just add `-forever`. Even if the scanner is unable to reach the server (i.e. the server is down) the scanner will continue to send out data. If the server does come back on, then it will automatically be reconnected. If you'd like to have the scanner run in the background forever you can prefix with `nohup` and suffix with `&`. 

Finally, the basic command then becomes:

```
$ nohup find3-cli-scanner -i YOURINTERFACE \
    -device YOURDEVICE -family YOURFAMILY \
    -server https://cloud.internalpositioning.com \
    -scantime 10 -bluetooth -forever &
```

### Passive scanning 

In *passive scanning* the scanner will report the classified location of the devices that it scans. This mode requires having a WiFi card that supports monitor mode. There are a number of possible USB WiFi adapters that support monitor mode. Here's a list that are popular:

- [USB Rt3070 $14](https://www.amazon.com/gp/product/B00NAXX40C/ref=as_li_tl?ie=UTF8&tag=scholl-20&camp=1789&creative=9325&linkCode=as2&creativeASIN=B00NAXX40C&linkId=b72d3a481799c15e483ea93c551742f4)
- [Panda PAU5 $14](https://www.amazon.com/gp/product/B00EQT0YK2/ref=as_li_tl?ie=UTF8&tag=scholl-20&camp=1789&creative=9325&linkCode=as2&creativeASIN=B00EQT0YK2&linkId=e5b954672d93f1e9ce9c9981331515c4)
- [Panda PAU6 $15](https://www.amazon.com/gp/product/B00JDVRCI0/ref=as_li_tl?ie=UTF8&tag=scholl-20&camp=1789&creative=9325&linkCode=as2&creativeASIN=B00JDVRCI0&linkId=e73e93e020941cada0e64b92186a2546)
- [Panda PAU9 $36](https://www.amazon.com/gp/product/B01LY35HGO/ref=as_li_tl?ie=UTF8&tag=scholl-20&camp=1789&creative=9325&linkCode=as2&creativeASIN=B01LY35HGO&linkId=e63f3beda9855abd59009d6173234918)
- [Alfa AWUSO36NH $33](https://www.amazon.com/gp/product/B0035APGP6/ref=as_li_tl?ie=UTF8&tag=scholl-20&camp=1789&creative=9325&linkCode=as2&creativeASIN=B0035APGP6&linkId=b4e25ba82357ca6f1a33cb23941befb3)
- [Alfa AWUS036NHA $40](https://www.amazon.com/gp/product/B004Y6MIXS/ref=as_li_tl?ie=UTF8&tag=scholl-20&camp=1789&creative=9325&linkCode=as2&creativeASIN=B004Y6MIXS&linkId=0277ca161967134a7f75dd7b3443bded)
- [Alfa AWUS036NEH $40](https://www.amazon.com/gp/product/B0035OCVO6/ref=as_li_tl?ie=UTF8&tag=scholl-20&camp=1789&creative=9325&linkCode=as2&creativeASIN=B0035OCVO6&linkId=bd45697540120291a2f6e169dcf81b96)
- [Sabrent NT-WGHU $15 (b/g) only](https://www.amazon.com/gp/product/B003EVO9U4/ref=as_li_tl?ie=UTF8&tag=scholl-20&camp=1789&creative=9325&linkCode=as2&creativeASIN=B003EVO9U4&linkId=06d4784d38b6bcef5957f3f6e74af8c8)

Namely you want to find a USB adapter with one of the following chipsets: Atheros AR9271, Ralink RT3070, Ralink RT3572, or Ralink RT5572.
You can simply run the command above with the flag `-passive` added to enable passive scanning.

```
$ nohup find3-cli-scanner -i YOURINTERFACE \
    -device YOURDEVICE -family YOURFAMILY \
    -server https://cloud.internalpositioning.com \
    -scantime 10 -bluetooth -forever -passive &
```

The above command will start by enabling monitor mode of the specified interface, then run the scan (using `tshark` and the bluetooth adapter), and then it will disable monitor mode so that the scan can be uploaded to the server. The enabling/disabling of monitor mode requires about 10 seconds each time. To remove this step you can enable monitor mode permanently.

```
$ find3-cli-scanner -i YOURINTERFACE -monitor-mode
```

After enabling monitor moe permanently you need to add a flag `-no-modify` to tell the command line scanner not to enable/disable automatically.

```
$ nohup find3-cli-scanner -i YOURINTERFACE \
    -device YOURDEVICE -family YOURFAMILY \
    -server https://cloud.internalpositioning.com \
    -scantime 10 -bluetooth -forever -passive -no-modify &
```

## Issues?

If you have issues, please [file a bug report on Github](https://github.com/schollz/find3-cli-scanner/issues/new?template=bugs.md&title=Bug:%20).

## Source

If you are interested, the app is completely open-source and available at  https://github.com/schollz/find3-cli-scanner.

## LICENSE

MIT
