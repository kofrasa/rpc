all: clean build test

SRC = ./...

build:
	@go build ${SRC}

clean:
	@go clean

test:
	@go test -v -cover ${SRC}

bench:
	@go test ${SRC} -bench=.

vet:
	@go vet ${SRC}


.PHONY: vet clean build test