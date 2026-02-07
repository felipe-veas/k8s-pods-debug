.PHONY: build test clean

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
BINARY_NAME=kpdbug

all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v cmd/kpdbug/main.go

test:
	$(GOTEST) -v ./pkg/...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
