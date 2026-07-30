.PHONY: build test lint clean

build:
	go build -o bin/glyph ./cmd/glyph

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin
