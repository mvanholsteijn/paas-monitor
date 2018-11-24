include Makefile.mk
REGISTRY_HOST=docker.io
USERNAME=mvanholsteijn
NAME=paas-monitor

IMAGE=$(REGISTRY_HOST)/$(USERNAME)/$(NAME)

post-release: check-release
	if [[ -z $(GITHUB_API_TOKEN) ]] ; then echo "ERROR: GITHUB_API_TOKEN not set." ; exit 1 ; fi
	./release-to-github
