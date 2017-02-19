FROM 		scratch
ADD 		public /app/public/
ENV		APPDIR /app
ADD		paas-monitor-linux-amd64 /paas-monitor
ADD		envconsul /

ENV		SERVICE_NAME paas-monitor
ENV		SERVICE_TAGS http

ENTRYPOINT 	[ "/paas-monitor" ]
EXPOSE 		1337
