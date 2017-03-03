include Makefile.mk
REGISTRY_HOST=docker.io
USERNAME=mvanholsteijn
NAME=paas-monitor

IMAGE=$(REGISTRY_HOST)/$(USERNAME)/$(NAME)


pre-build: paas-monitor marathon-lb.json

post-release: check-release	
	docker tag  -f gcr.io/instruqt/$(NAME):$(VERSION) $(IMAGE):$(VERSION)
	docker tag  -f gcr.io/instruqt/$(NAME):latest $(IMAGE):$(VERSION)
	docker push gcr.io/instruqt/$(NAME):$(VERSION)
	docker push gcr.io/instruqt/$(NAME):latest
	if [[ -z $(GITHUB_API_TOKEN) ]] ; then echo "ERROR: GITHUB_API_TOKEN not set." ; exit 1 ; fi
	./release-to-github

paas-monitor: paas-monitor.go
	docker run --rm \
	-v $(PWD):/src \
	centurylink/golang-builder

envconsul:
	curl -L https://github.com/hashicorp/envconsul/releases/download/v0.5.0/envconsul_0.5.0_linux_amd64.tar.gz  | tar --strip-components=1 -xvzf -

marathon-lb.json: marathon.json
	jq '. + { "labels": {"HAPROXY_GROUP":"external", "HAPROXY_0_VHOST":"paas-monitor.127.0.0.1.xip.io"}}' marathon.json > marathon-lb.json


clean:
	rm -rf paas-monitor envconsul
