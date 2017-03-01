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
# Native 
go run paas-monitor.go &
open http://0.0.0.0:1337

# Docker
docker run -d -p 1337:1337 mvanholsteijn/paas-monitor:latest
open http://0.0.0.0:1337

# in Docker container via Marathon
curl -X POST -d @marathon-docker.json http://localhost:8888/v2/apps

# Marathon 
curl -X POST -d @marathon.json http://localhost:8888/v2/apps
```

## Running in a PaaS

When you are deploying your application in a PaaS, like CloudFoundry or Marathon the platform
will pass in the port to listen on via an environment variable. using the option `-port-env-name`
you can specify the name of that environment variable. The following table shows possible
port environment variable names per platform.


| Platform | name |
| -------- | -----|
| Marathon | PORT0 or PORT_1337 |

if no environment name is specified and the environment variable PORT is set, it will override
the default port of 1337.

On Marathon, you would start the paas-monitor by specifying:

```json
{
  ...
  "cmd": "paas-monitor -port-env-name PORT0"
  ...
}
```

## Application endpoints
The application has the following endpoints:

| URI | description |
| --- | ------------|
| /		| services the web UI|
| /status	| called by the web UI, returning hostname,message, release and server count|
| /environment	| returns the environment variables of the server process|
| /header	| returns the HTTP headers received by the server|
| /request	| returns the HTTP request properties received by the server|
| /health	| returns ok|
| /stop		| exits the application|

