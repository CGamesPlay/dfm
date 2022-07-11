.PHONY: all install test release

SOURCES = $(wildcard *.go) go.mod go.sum
# Experiment with go build -ldflags="-X 'main.Version=v1.0.0'"
GOFLAGS_debug = -ldflags '-X "main.Version=$(shell git rev-parse --short HEAD; [ -z "$$(git status --porcelain --untracked-files=no)" ] || echo 'with uncommitted changes')"'
GOFLAGS_release = -ldflags '-s -w -extldflags "-static" -X "main.Version=$(shell cat VERSION)"'

all: install

install:
	go install $(GOFLAGS_debug) .

bin/darwin_amd64/dfm: $(SOURCES)
	mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $@ $(GOFLAGS_release) .

bin/darwin_arm64/dfm: $(SOURCES)
	mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $@ $(GOFLAGS_release) .

bin/linux_amd64/dfm: $(SOURCES)
	mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(GOFLAGS_release) .

bin/linux_arm/dfm: $(SOURCES)
	mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o $@ $(GOFLAGS_release) .

bin/%.tar.gz: bin/%/dfm
	tar -czf $@ -C $(dir $^) $(notdir $^)

release: bin/darwin_amd64.tar.gz bin/darwin_arm64.tar.gz bin/linux_amd64.tar.gz bin/linux_arm.tar.gz

test: install
	go test . -tags=integration

