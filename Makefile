# Parameters to compile and run application
GOOS?=linux
GOARCH?=amd64

# Current version and commit
VERSION=`git describe --tags`
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags="-X main.version=$(VERSION)/$(BUILD_TIME)"
APP="athenalyzer"

lint:
	@echo "+ $@"
	@for f in $(find -name "*.go" | grep -v "vendor\/"); do \
		golint $f; \
	done

fmt:
	@echo "+ $@"
	@gofmt -w ./

tidy:
	@echo "+ $@"
	@set -e; export GOLANGFLAGS="-mod=vendor"; \
	go mod tidy

.PHONY: vendor
vendor:
	@echo "+ $@"
	@go mod vendor

# Compile application
build: tidy vendor fmt lint linux-amd64 windows-amd64

linux-amd64:
	@echo "+ $@"
	@set -e; export GOOS=linux; export GOARCH=amd64; \
	go build $(LDFLAGS) -mod vendor -o ./$(APP)-$@ ./cmd/$(APP)

windows-amd64:
	@echo "+ $@"
	@set -e; export GOOS=windows; export GOARCH=amd64; \
	go build $(LDFLAGS) -mod vendor -o ./$(APP)-$@ ./cmd/$(APP)

clean:
	@rm -vf ./$(APP)-* ./$(APP)

install: build
	@mkdir -p $(HOME)/bin
	@cp -Rv ./$(APP)-linux-amd64 $(HOME)/bin/$(APP)

uninstall:
	@rm -vf $(HOME)/bin/$(APP)

rebuild: clean build
