include Makefile.mk
REGISTRY_HOST=docker.io
USERNAME=mvanholsteijn
NAME=paas-monitor

IMAGE=$(REGISTRY_HOST)/$(USERNAME)/$(NAME)


pre-build: paas-monitor-linux-amd64 marathon-lb.json

post-release: check-release	
	[[ -n $(GITHUB_API_TOKEN) ]] && echo "ERROR: GITHUB_API_TOKEN not set." && exit 1
	./release-to-github

paas-monitor-linux-amd64: paas-monitor.go
	mkdir -p binaries
	docker run --rm \
	-e BUILD_GOOS="linux" \
	-e BUILD_GOARCH="amd64" \
	-v $(PWD):/src \
	centurylink/golang-builder-cross

envconsul:
	curl -L https://github.com/hashicorp/envconsul/releases/download/v0.5.0/envconsul_0.5.0_linux_amd64.tar.gz  | tar --strip-components=1 -xvzf -

marathon-lb.json: marathon.json
	jq '. + { "labels": {"HAPROXY_GROUP":"external", "HAPROXY_0_VHOST":"paas-monitor.127.0.0.1.xip.io"}}' marathon.json > marathon-lb.json

clean:
	rm -rf paas-monitor envconsul
