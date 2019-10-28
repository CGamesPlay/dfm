.PHONY: test release

SOURCES = $(wildcard *.go) go.mod go.sum
GOFLAGS = -ldflags '-s -w -extldflags "-static"'

bin/dfm: $(SOURCES)
	mkdir -p bin
	go build -o bin/dfm .

bin/darwin_amd64/dfm: $(SOURCES)
	mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $@ $(GOFLAGS) .

bin/linux_amd64/dfm: $(SOURCES)
	mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(GOFLAGS) .

bin/%.tar.gz: bin/%/dfm
	tar -czf $@ -C $(dir $^) $(notdir $^)

release: bin/darwin_amd64.tar.gz bin/linux_amd64.tar.gz

test: bin/dfm
	go test .
	./test/snapshot.sh

