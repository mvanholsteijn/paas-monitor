paas-monitor: paas-monitor.go
	docker run --rm -v $$(pwd):/src -v /var/run/docker.sock:/var/run/docker.sock centurylink/golang-builder

clean:
	rm -rf paas-monitor

release: paas-monitor
	docker build -t mvanholsteijn/paas-monitor:latest . 
	docker push mvanholsteijn/paas-monitor:lastest
