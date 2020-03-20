# Parameters to compile and run application
GOOS?=linux
GOARCH?=amd64

# Current version and commit
VERSION=`git describe --tags`
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags="-X main.version=$(VERSION)/$(BUILD_TIME)"
APP="athenalyzer"


fmt:
	@echo "+ $@"
	@gofmt -w ./

# Compile application
build: fmt app

app:
	@echo "+ $@"
	@set -e; export GOFLAGS="-mod=vendor"; go build $(LDFLAGS) ./cmd/$(APP)


clean:
	@rm -vf $(APP)

install: $(APP)
	@mkdir -p $(HOME)/bin
	@cp -Rv ./$(APP) $(HOME)/bin/ 

uninstall:
	@rm -vf $(HOME)/bin/$(APP)

rebuild: clean build
