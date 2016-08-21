PLATFORMS= \
	freebsd-amd64 \
	linux-amd64 \
	windows-amd64 \
	darwin-amd64 \
	$(NULL)

NAME=slipscheme

OS=$(shell uname -s)
ifeq ($(filter CYGWIN%,$(OS)),$(OS))
	export CWD=$(shell cygpath -wa .)
	export SEP=\\
	export CYGWIN=winsymlinks:native
	BIN ?= $(GOBIN)$(SEP)$(NAME).exe
else
	export CWD=$(shell pwd)
	export SEP=/
	BIN ?= $(GOBIN)$(SEP)$(NAME)
endif

export GOPATH=$(CWD)

DIST=$(CWD)$(SEP)dist

GOBIN ?= $(CWD)

CURVER ?= $(patsubst v%,%,$(shell [ -d .git ] && git describe --abbrev=0 --tags || grep ^\#\# CHANGELOG.md | awk '{print $$2; exit}'))
LDFLAGS:=-X jira.VERSION=$(CURVER) -w

# use make DEBUG=1 and you can get a debuggable golang binary
# see https://github.com/mailgun/godebug
ifneq ($(DEBUG),)
	GOBUILD=go get -v github.com/mailgun/godebug && ./bin/godebug build
else
	GOBUILD=go build -v -ldflags "$(LDFLAGS) -s"
endif

build: 
	$(GOBUILD) -o '$(BIN)' $(NAME).go

debug:
	$(MAKE) DEBUG=1

vet:
	@go tool vet *.go

cross-setup:
	for p in $(PLATFORMS); do \
        echo Building for $$p"; \
		cd $(GOROOT)/src && sudo GOROOT_BOOTSTRAP=$(GOROOT) GOOS=$${p/-*/} GOARCH=$${p/*-/} bash ./make.bash --no-clean; \
   done

all:
	rm -rf $(DIST); \
	mkdir -p $(DIST); \
	for p in $(PLATFORMS); do \
        echo "Building for $$p"; \
        ${MAKE} build GOOS=$${p/-*/} GOARCH=$${p/*-/} BIN=$(DIST)/$(NAME)-$$p; \
    done
	shopt -s nullglob; for x in m$(DIST)/$(NAME)-windows-*; do mv $$x $$x.exe; done

fmt:
	gofmt -s -w *.go

install:
	${MAKE} GOBIN=$$HOME/bin build

NEWVER ?= $(shell echo $(CURVER) | awk -F. '{print $$1"."$$2"."$$3+1}')
TODAY  := $(shell date +%Y-%m-%d)

changes:
	@git log --pretty=format:"* %s [%cn] [%h]" --no-merges ^v$(CURVER) HEAD *.go | grep -vE 'gofmt|go fmt'

update-changelog:
	@echo "# Changelog" > CHANGELOG.md.new; \
	echo >> CHANGELOG.md.new; \
	echo "## $(NEWVER) - $(TODAY)" >> CHANGELOG.md.new; \
	echo >> CHANGELOG.md.new; \
	$(MAKE) --no-print-directory --silent changes | \
	perl -pe 's{\[([a-f0-9]+)\]}{[[$$1](https://github.com/coryb/slipscheme/commit/$$1)]}g' | \
	perl -pe 's{\#(\d+)}{[#$$1](https://github.com/coryb/slipscheme/issues/$$1)}g' >> CHANGELOG.md.new; \
	tail -n +2 CHANGELOG.md >> CHANGELOG.md.new; \
	mv CHANGELOG.md.new CHANGELOG.md; \
	git commit -m "Updated Changelog" CHANGELOG.md; \
	git tag v$(NEWVER)

version:
	@echo $(CURVER)

clean:
	rm -rf pkg src dist bin ./$(NAME)

GOTARGZ=go1.6.3.linux-amd64.tar.gz
docker-build:
	${MAKE} PLATFORMS=linux-amd64 all
	mkdir -p docker-root/bin
	cp dist/slipscheme-linux-amd64 docker-root/bin/slipscheme
	[ -f $(GOTARGZ) ] || wget https://storage.googleapis.com/golang/$(GOTARGZ)
	tar xzf ./go1.6.3.linux-amd64.tar.gz -C docker-root --strip-components 1 go/bin/gofmt
	docker build -t coryb/$(NAME):$(CURVER) .
	docker tag coryb/$(NAME):$(CURVER) coryb/$(NAME):latest

docker-release: docker-build
	docker push coryb/$(NAME):$(CURVER)
	docker push coryb/$(NAME):latest

