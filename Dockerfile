FROM 		golang:1.8 

WORKDIR		/
ADD		paas-monitor.go /
RUN		CGO_ENABLED=0 GOOS=linux go get github.com/shirou/gopsutil
RUN		CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' paas-monitor.go
RUN		curl -sS -L https://releases.hashicorp.com/envconsul/0.7.2/envconsul_0.7.2_linux_amd64.tgz | \
			tar -xvzf -

FROM 		scratch
ARG		VERSION

ADD 		public /app/public/
COPY --from=0		/paas-monitor /
COPY --from=0		/envconsul /

ENV		APPDIR /app \
		SERVICE_NAME paas-monitor \
		SERVICE_TAGS http \
		PORT 1337 \
		RELEASE=${VERSION}

ENTRYPOINT 	["/paas-monitor"]
EXPOSE 		1337
HEALTHCHECK	--retries=1 --start-period=3s --interval=30s --timeout=3s CMD  ["/paas-monitor", "-check"]
