include Makefile.mk
REGISTRY_HOST=docker.io
USERNAME=mvanholsteijn
NAME=paas-monitor

IMAGE=$(REGISTRY_HOST)/$(USERNAME)/$(NAME)

post-build:
	ID=$$(docker create $(IMAGE):$(VERSION)); docker cp $$ID:/paas-monitor paas-monitor-linux-amd64; docker rm $$ID

post-release: check-release
	if [[ -z $(GITHUB_API_TOKEN) ]] ; then echo "ERROR: GITHUB_API_TOKEN not set." ; exit 1 ; fi
	./release-to-github
