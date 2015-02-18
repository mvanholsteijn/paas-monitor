FROM 		golang
ADD 		. /go/src/github.com/mvanholsteijn/paas-monitor
RUN 		go install github.com/mvanholsteijn/paas-monitor
ENTRYPOINT 	/go/bin/paas-monitor

# Document that the service listens on port 1337
EXPOSE 		1337
