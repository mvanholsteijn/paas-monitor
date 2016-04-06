include Makefile.mk
REGISTRY_HOST=docker.io
USERNAME=mvanholsteijn
NAME=paas-monitor

IMAGE=$(REGISTRY_HOST)/$(USERNAME)/$(NAME)


pre-build: paas-monitor


paas-monitor: paas-monitor.go envconsul
	docker run --rm -v $$(pwd):/src -v /var/run/docker.sock:/var/run/docker.sock centurylink/golang-builder

envconsul:
	curl -L https://github.com/hashicorp/envconsul/releases/download/v0.5.0/envconsul_0.5.0_linux_amd64.tar.gz  | tar --strip-components=1 -xvzf -

clean:
	rm -rf paas-monitor envconsul
