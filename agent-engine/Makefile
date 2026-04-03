.PHONY: build test lint fmt run dev tui docker release clean

BINARY := agent-engine
CMD := ./cmd/agent-engine
BUILD_DIR := bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

dev:
	go run $(CMD) -port 8080

tui:
	go run $(CMD)

test:
	go test ./...

test-cover:
	go test -coverprofile=cover.out ./...
	go tool cover -html=cover.out -o cover.html

lint:
	golangci-lint run

fmt:
	go fmt ./...
	goimports -w .

tidy:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR) cover.out cover.html

release:
	GOOS=linux   GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-linux-amd64   $(CMD)
	GOOS=darwin  GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-darwin-amd64  $(CMD)
	GOOS=darwin  GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY)-darwin-arm64  $(CMD)
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe $(CMD)

docker:
	docker build -t $(BINARY):latest .
