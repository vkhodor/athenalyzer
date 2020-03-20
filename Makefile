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
	@go build $(LDFLAGS) ./cmd/$(APP)

Gopkg.toml:
	@$(GOPATH)/bin/dep init

clean:
	@rm -vrf vendor
	@rm -vf Gopkg.*
	@rm -vf $(APP)

install: $(APP)
	@mkdir -p $(HOME)/bin
	@cp -Rv ./$(APP) $(HOME)/bin/ 

uninstall:
	@rm -vf $(HOME)/bin/$(APP)

rebuild: Gopkg.toml
	@rm -vf ./$(APP)
	@go build $(LDFLAGS) ./cmd/$(APP)

