.PHONY: test

bin/dfm: $(wildcard *.go) go.mod go.sum
	mkdir -p bin
	go build -o bin/dfm .

test: bin/dfm
	go test .
	./test/snapshot.sh

