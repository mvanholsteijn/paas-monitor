FROM 		scratch
ADD 		public /app/public/
ENV		APPDIR /app
ADD		paas-monitor /
ADD		envconsul /
ENTRYPOINT 	[ "/paas-monitor" ]
EXPOSE 		1337
