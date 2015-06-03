build: paas-monitor envconsul
	@ [ -z "$$(git status -s)" ] || (echo "outstanding changes" ; git status -s) && exit 1
	docker build -t mvanholsteijn/paas-monitor:$$(git rev-parse --short HEAD) . 
	docker tag  -f mvanholsteijn/paas-monitor:$$(git rev-parse --short HEAD) mvanholsteijn/paas-monitor:latest  

release: build
	docker push mvanholsteijn/paas-monitor:$$(git rev-parse --short HEAD)
	docker push mvanholsteijn/paas-monitor:latest

paas-monitor: paas-monitor.go
	docker run --rm -v $$(pwd):/src -v /var/run/docker.sock:/var/run/docker.sock centurylink/golang-builder

envconsul: 
	curl -L https://github.com/hashicorp/envconsul/releases/download/v0.5.0/envconsul_0.5.0_linux_amd64.tar.gz  | \
		tar --strip-components=1 -xvzf -
clean:
	rm -rf paas-monitor envconsul

