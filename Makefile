BINARY   := codeaudit
CMD      := ./cmd/codeaudit
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.version=$(VERSION)

.PHONY: build lint test clean cross

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

# Static binary for Linux (e.g. Docker)
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 $(CMD)

# Cross-compile targets
cross: build-linux
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-amd64 $(CMD)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64 $(CMD)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-windows-amd64.exe $(CMD)

lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

test:
	go test ./... -v -count=1

clean:
	rm -f $(BINARY) $(BINARY)-*

deps:
	go mod tidy
	go mod download
