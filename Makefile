GOFLAGS := -trimpath

.PHONY: run build build-all test fmt format-check vet check clean clean-dev seed dev

run:
	go run ./cmd/mess

seed:
	go run ./cmd/mess seed --db .data/mess.db

dev:
	go run ./cmd/mess --db .data/mess.db

clean-dev:
	rm -rf .data

build:
	go build $(GOFLAGS) -o bin/mess ./cmd/mess

build-all:
	GOOS=darwin  GOARCH=arm64 go build $(GOFLAGS) -o bin/mess-darwin-arm64 ./cmd/mess
	GOOS=linux   GOARCH=amd64 go build $(GOFLAGS) -o bin/mess-linux-amd64 ./cmd/mess
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o bin/mess-windows-amd64.exe ./cmd/mess

test:
	go test ./...

fmt:
	gofmt -w cmd internal

format-check:
	@files="$$(gofmt -l cmd internal)"; if [ -n "$$files" ]; then printf '%s\n' "$$files"; exit 1; fi

vet:
	go vet ./...

check: format-check vet test build

clean:
	rm -rf bin
