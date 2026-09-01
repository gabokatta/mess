GOFLAGS := -trimpath

.PHONY: run build build-all test fmt clean

run:
	go run ./cmd/mes

build:
	go build $(GOFLAGS) -o bin/mes ./cmd/mes

build-all:
	GOOS=darwin  GOARCH=arm64 go build $(GOFLAGS) -o bin/mes-darwin-arm64 ./cmd/mes
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o bin/mes-windows-amd64.exe ./cmd/mes

test:
	go test ./...

fmt:
	gofmt -w .
	go vet ./...

clean:
	rm -rf bin
