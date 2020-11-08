GOBUILD=CGO_ENABLED=0 go build -ldflags "-w -s"
build:
	 $(GOBUILD) $(if $(GOOS),-o dist/slipscheme-$(GOOS)-$(GOARCH)$(if $(filter windows,$(GOOS)),.exe,),)

all:
	@rm -rf dist && mkdir -p dist
	@docker run --rm -v $(PWD):/src -w /src -e GOOS=freebsd -e GOARCH=amd64 golang:latest make build
	@docker run --rm -v $(PWD):/src -w /src -e GOOS=linux -e GOARCH=amd64 golang:latest make build
	@docker run --rm -v $(PWD):/src -w /src -e GOOS=windows -e GOARCH=amd64 golang:latest make build
	@docker run --rm -v $(PWD):/src -w /src -e GOOS=darwin -e GOARCH=amd64 golang:latest make build

install:
	$(GOBUILD) -o $(HOME)/bin/

CURVER ?= $(patsubst v%,%,$(shell [ -d .git ] && git describe --abbrev=0 --tags || grep ^\#\# CHANGELOG.md | awk '{print $$2; exit}'))
NEWVER ?= $(shell echo $(CURVER) | awk -F. '{print $$1"."$$2"."$$3+1}')
TODAY  := $(shell date +%Y-%m-%d)

changes:
	@git log --pretty=format:"* %s [%cn] [%h]" --no-merges ^v$(CURVER) HEAD $$(git ls-files | grep [.]go$ | grep -v _test[.]go ) go.mod | \
		perl -pe 's{\[([a-f0-9]+)\]}{[[$$1](https://github.com/coryb/slipscheme/commit/$$1)]}g' | \
		perl -pe 's{\#(\d+)}{[#$$1](https://github.com/coryb/slipscheme/issues/$$1)}g'; 

update-changelog:
	@echo "# Changelog" > CHANGELOG.md.new; \
	echo >> CHANGELOG.md.new; \
	echo "## $(NEWVER) - $(TODAY)" >> CHANGELOG.md.new; \
	echo >> CHANGELOG.md.new; \
	$(MAKE) --no-print-directory --silent changes >> CHANGELOG.md.new; \
	tail -n +2 CHANGELOG.md >> CHANGELOG.md.new; \
	mv CHANGELOG.md.new CHANGELOG.md; \
	git commit -m "Updated Changelog" CHANGELOG.md; \
	git tag -f v$(NEWVER)

version:
	@echo $(CURVER)

IMAGE=$(patsubst git@github.com:%,%,$(patsubst https://github.com/%,%,$(patsubst %.git,%,$(shell git ls-remote --get-url))))

docker-build:
	docker build \
		--build-arg VERSION=$(CURVER) \
		--build-arg VCS_REF=$(strip $(shell git rev-parse --short HEAD)) \
		-t $(IMAGE):$(CURVER) .
	docker tag $(IMAGE):$(CURVER) $(IMAGE):latest

docker-release: docker-build
	docker push $(IMAGE):$(CURVER)
	docker push $(IMAGE):latest

