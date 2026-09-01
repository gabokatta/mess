GOFLAGS := -trimpath

.PHONY: run build build-all test fmt clean

run:
	go run ./cmd/mess

build:
	go build $(GOFLAGS) -o bin/mess ./cmd/mess

build-all:
	GOOS=darwin  GOARCH=arm64 go build $(GOFLAGS) -o bin/mess-darwin-arm64 ./cmd/mess
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o bin/mess-windows-amd64.exe ./cmd/mess

test:
	go test ./...

fmt:
	gofmt -w .
	go vet ./...

clean:
	rm -rf bin
