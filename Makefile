.PHONY: run build test tidy fmt vet

# Start the panel locally (http://localhost:8080).
run:
	go run ./cmd/panel

# Build all binaries into ./bin.
build:
	go build -o bin/panel ./cmd/panel
	go build -o bin/node ./cmd/node
	go build -o bin/wisp-certs ./cmd/wisp-certs

# Run all tests.
test:
	go test ./...

# Tidy module dependencies.
tidy:
	go mod tidy

# Format and vet the code.
fmt:
	go fmt ./...

vet:
	go vet ./...
