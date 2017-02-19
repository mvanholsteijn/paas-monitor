# paas-monitor
An monitoring application to observe the behaviour of PaaS platforms.

It can be used to see  how the PaaS platform handles:

- [rolling upgrades.](http://blog.xebia.com/2015/04/06/rolling-upgrade-of-docker-applications-using-coreos-and-consul/)
- scaling
- application crashes
- machines crashes
- http requests

## running the application

to run the application, type one of:

```
# native 
go run paas-monitor.go &
open http://0.0.0.0:1337

# docker
docker run -d -p 1337:1337 mvanholsteijn/paas-monitor:latest
open http://0.0.0.0:1337

# marathon
curl -X POST -d @marathon.json http://localhost:8888/v2/apps
```


## Application endpoints
The application has the following endpoints:

| URI | description |
+ --- + ------------+
| /		| services the web UI|
| /status	| called by the web UI, returning hostname,message, release and server count|
| /environment	| returns the environment variables of the server process|
| /header	| returns the HTTP headers received by the server|
| /request	| returns the HTTP request properties received by the server|
| /health	| returns ok|
| /stop		| exits the application|

