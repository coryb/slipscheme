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
GOBUILD=go build -v -ldflags "$(LDFLAGS) -s"

build: 
	$(GOBUILD) -o '$(BIN)' schemy.go

src/%:
	mkdir -p $(@D)
	test -L $@ || ln -sf '$(GOPATH)' $@
	go get -v $* $*/main

vet:
	@go tool vet *.go

all: src/$(PACKAGE)
	docker pull karalabe/xgo-latest
	rm -rf dist
	mkdir -p dist
	docker run --rm -e EXT_GOPATH=/gopath -v $$(pwd):/gopath -e TARGETS="$(PLATFORMS)" -v $$(pwd)/dist:/build karalabe/xgo-latest $(PACKAGE)/main
	[ `uname -s` = 'Darwin' ] && format=-f || format=-c; \
	eval $$(stat $$format "uid=%u gid=%g" .); \
	for x in $(DIST)/main-*; do \
		y=$$(echo $$x | sed -e 's/darwin-[^-]*/darwin/' -e 's/main/newt/'); \
		mv $$x $$y; \
		eval $$(stat $$format "binuid=%u bingid=%g" $$y); \
		if [ "$$binuid" != "$$uid" -o "$$bingid" != "$$gid" ]; then \
			sudo chown $$uid:$$gid $$y; \
		fi \
	done

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
	rm -rf pkg dist bin src ./$(NAME)
