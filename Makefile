BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
APPNAME := gnodi

# do not override user values
ifeq (,$(VERSION))
  VERSION := $(shell git describe --exact-match 2>/dev/null)
  # if VERSION is empty, then populate it with branch name and raw commit hash
  ifeq (,$(VERSION))
    VERSION := $(BRANCH)-$(COMMIT)
  endif
endif

# Update the ldflags with the app, client & server names
ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=$(APPNAME) \
	-X github.com/cosmos/cosmos-sdk/version.AppName=$(APPNAME)d \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT)

BUILD_FLAGS := $(ldflags)

##############
###  Test  ###
##############

test-unit:
	@echo Running unit tests...
	@go test -mod=readonly -v -timeout 30m ./...

test-race:
	@echo Running unit tests with race condition reporting...
	@go test -mod=readonly -v -race -timeout 30m ./...

test-cover:
	@echo Running unit tests and creating coverage report...
	@go test -mod=readonly -v -timeout 30m -coverprofile=$(COVER_FILE) -covermode=atomic ./...
	@go tool cover -html=$(COVER_FILE) -o $(COVER_HTML_FILE)
	@rm $(COVER_FILE)

bench:
	@echo Running unit tests with benchmarking...
	@go test -mod=readonly -v -timeout 30m -bench=. ./...

test: govet govulncheck test-unit

.PHONY: test test-unit test-race test-cover bench

#################
###  Install  ###
#################

all: install

install:
	@echo "--> ensure dependencies have not been modified"
	@go mod verify
	@echo "--> installing $(APPNAME)d"
	@go install $(BUILD_FLAGS) -mod=readonly ./cmd/$(APPNAME)d

.PHONY: all install

##################
###  Protobuf  ###
##################

# Use this target if you do not want to use Ignite for generating proto files

proto-deps:
	@echo "Installing proto deps"
	@echo "Proto deps present, run 'go tool' to see them"

proto-gen:
	@echo "Generating protobuf files..."
	@ignite generate proto-go --yes

.PHONY: proto-gen

#################
###  Linting  ###
#################

lint:
	@echo "--> Running linter"
	@go tool github.com/golangci/golangci-lint/cmd/golangci-lint run ./... --timeout 15m

lint-fix:
	@echo "--> Running linter and fixing issues"
	@go tool github.com/golangci/golangci-lint/cmd/golangci-lint run ./... --fix --timeout 15m

.PHONY: lint lint-fix

###################
### Development ###
###################

govet:
	@echo Running go vet...
	@go vet ./...

govulncheck:
	@echo Running govulncheck...
	@go tool golang.org/x/vuln/cmd/govulncheck@latest
	@govulncheck ./...

.PHONY: govet govulncheck

#################
###  Build  ###
#################

# Build for Linux (AMD64)
build-linux:
	env GOOS=linux GOARCH=amd64 go build -ldflags '$(BUILD_FLAGS) -X github.com/cosmos/cosmos-sdk/version.BuildTags=linux,amd64' -o ./build/gnodi-linux ./cmd/gnodid/main.go

# Build for macOS (Apple Silicon)
build-darwin-arm64:
	env GOOS=darwin GOARCH=arm64 go build -ldflags '$(BUILD_FLAGS) -X github.com/cosmos/cosmos-sdk/version.BuildTags=darwin,arm64' -o ./build/gnodi-darwin-arm64 ./cmd/gnodid/main.go

# Build for macOS (Intel)
build-darwin-amd64:
	env GOOS=darwin GOARCH=amd64 go build -ldflags '$(BUILD_FLAGS) -X github.com/cosmos/cosmos-sdk/version.BuildTags=darwin,amd64' -o ./build/gnodi-darwin-amd64 ./cmd/gnodid/main.go

# Build for Windows (AMD64)
build-windows:
	env GOOS=windows GOARCH=amd64 go build -ldflags '$(BUILD_FLAGS) -X github.com/cosmos/cosmos-sdk/version.BuildTags=windows,amd64' -o ./build/gnodi-windows.exe ./cmd/gnodid/main.go

# Build for all platforms
build: build-linux build-darwin-arm64 build-darwin-amd64 build-windows