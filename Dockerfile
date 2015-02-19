FROM 		golang
ADD 		public /app/public/
ADD 		paas-monitor.go /go/src/github.com/mvanholsteijn/paas-monitor/
RUN 		go install github.com/mvanholsteijn/paas-monitor

ENV		APPDIR /app
ENTRYPOINT 	/go/bin/paas-monitor
EXPOSE 		1337
