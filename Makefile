# Variables
BINARY_NAME=kubectl-setimg
VERSION ?= dev
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG = $(shell git describe --tags --exact-match 2>/dev/null || echo "")

# Build flags
LDFLAGS = -ldflags "\
	-X main.Version=$(VERSION) \
	-X main.GitCommit=$(GIT_COMMIT) \
	-X main.GitTag=$(GIT_TAG)"

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)