.PHONY: build install test clean

build:
	go build -o wt ./cmd/wt

install:
	go install ./cmd/wt

test:
	go test ./...

clean:
	rm -f wt
