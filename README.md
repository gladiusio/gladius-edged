# Gladius Edged

All of the code and resources for running a Gladius edge daemon.

## Setup

#### Set up content delivery

Right now files are loaded from `~/.gladius/content/<website>/asset` on unix machines and `C:\Users\<user>\.gladius\content\<website>\asset` 

#### Config
This is the default config, you can override it by placing a file called `gladius-edged.toml` in the gladius base. You can also override any config value with an environment var like `EDGED_DISABLEAUTOJOIN=ture`
```toml
# Uncomment to override the detected content directory
# contentdirectory = "/your/path/here"

# What port to serve content on
contentport = "8080"

# How to reach the network gateway
networkgateayprotocol = "http"
networkgatewayhostname = "localhost"
networkgatewayport = "3001"

# Disable p2p join on start
disableautojoin = false

# Don't send a keep alive
disableheartbeat = false

# Change the base directory
gladiusbase = "/home/alex/.gladius"

# Log level printing
loglevel = "info"

# Override IP detection and use the IP specified as the accesible address for content
overrideip = ""

# What node we request to join at startup
p2pseednodeaddress = "165.227.16.209"
p2pseednodeport = "7947"
```
## Development
### Dependencies
To test and build the gladius-networkd binary you will need go-dep and make on your machine.

- Install [go](https://golang.org/doc/install)
- Enable go modules with `GO111MODULES=on`
- *Mac Users:* Install xcode for make `xcode-select --install`
- *Windows Users:* Install [Linux Subsystem](https://docs.microsoft.com/en-us/windows/wsl/install-win10)

### Build
If you updated the certificate files, run `packr`. You can learn how to install it [here](https://github.com/gobuffalo/packr)

To build the edged binary for your OS and architecture execute `make`.
After the build process you will find all binaries in *./build/*.


#### Build for a different platform
To build for a different platform specify toe GOOS and GOARCH variable.
```shell
# build for windows 64bit
GOOS=windows GOARCH=amd64 make

# build for linux 32bit
GOOS=linux GOARCH=386 make
```

---

## Some benchmarks compared to the previous version
Done over a gigabit link between two machines with the same bundle file being
served.

### Node version (with express routing)
```
ab -n 5000 -c 1000 http://<remote IP>:8080/content_bundle

This is ApacheBench, Version 2.3 <$Revision: 1796539 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking <remote IP> (be patient)
Finished 5000 requests


Server Software:        
Server Hostname:        <remote IP>
Server Port:            8080

Document Path:          /content_bundle
Document Length:        452460 bytes

Concurrency Level:      1000
Time taken for tests:   32.079 seconds
Complete requests:      5000
Failed requests:        0
Total transferred:      2263530000 bytes
HTML transferred:       2262300000 bytes
Requests per second:    155.87 [#/sec] (mean)
Time per request:       6415.760 [ms] (mean)
Time per request:       6.416 [ms] (mean, across all concurrent requests)
Transfer rate:          68907.77 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        4  268 611.6     39    3066
Processing:  1095 3654 3755.5   2974   31027
Waiting:       18 1029 3766.7     92   30872
Total:       1112 3922 3957.5   3072   32070

Percentage of the requests served within a certain time (ms)
  50%   3072
  66%   3402
  75%   3549
  80%   3596
  90%   5293
  95%   8859
  98%  18118
  99%  31110
 100%  32070 (longest request)
```
### Go version
```
ab -n 5000 -c 1000 http://<remote IP>:8080/content\?website\=test.com

This is ApacheBench, Version 2.3 <$Revision: 1796539 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking <remote IP> (be patient)

Finished 5000 requests


Server Software:        fasthttp
Server Hostname:        <remote IP>
Server Port:            8080

Document Path:          /content?website=test.com
Document Length:        452461 bytes

Concurrency Level:      1000
Time taken for tests:   19.265 seconds
Complete requests:      5000
Failed requests:        0
Total transferred:      2263050000 bytes
HTML transferred:       2262305000 bytes
Requests per second:    259.54 [#/sec] (mean)
Time per request:       3853.006 [ms] (mean)
Time per request:       3.853 [ms] (mean, across all concurrent requests)
Transfer rate:          114716.15 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        9  185 365.8     47    3089
Processing:    14 3534 458.6   3622    6552
Waiting:        3   76  99.7     48    2276
Total:         32 3719 637.0   3677    7604

Percentage of the requests served within a certain time (ms)
  50%   3677
  66%   3706
  75%   3749
  80%   3919
  90%   4662
  95%   4834
  98%   5144
  99%   5535
 100%   7604 (longest request)
```
As you can see above, the Go version handles high concurrent request loads
significantly better than the Node.js version of the network daemon. The Go
version also saturates the Gigabit link, which likely means it can provide even
more performance.
