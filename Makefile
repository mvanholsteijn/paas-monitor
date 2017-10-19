include Makefile.mk
REGISTRY_HOST=docker.io
USERNAME=mvanholsteijn
NAME=paas-monitor

IMAGE=$(REGISTRY_HOST)/$(USERNAME)/$(NAME)


pre-build: marathon-lb.json

post-release: check-release push-to-gcr
	if [[ -z $(GITHUB_API_TOKEN) ]] ; then echo "ERROR: GITHUB_API_TOKEN not set." ; exit 1 ; fi
	./release-to-github

push-to-gcr:
	docker tag $(IMAGE):$(VERSION) gcr.io/instruqt/$(NAME):$(VERSION)
	docker tag $(IMAGE):$(VERSION) gcr.io/instruqt/$(NAME):latest
	gcloud docker --project instruqt -- push gcr.io/instruqt/$(NAME):$(VERSION)
	gcloud docker --project instruqt -- push gcr.io/instruqt/$(NAME):latest

marathon-lb.json: marathon.json
	jq '. + { "labels": {"HAPROXY_GROUP":"external", "HAPROXY_0_VHOST":"paas-monitor.127.0.0.1.xip.io"}}' marathon.json > marathon-lb.json

