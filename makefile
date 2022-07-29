GIT_COMMIT := $(shell git describe --always --dirty)
GOPATH=$(shell go env GOPATH)/bin

build: format
	@echo '- build'
	@env go build -ldflags="-X github.com/ntorresalberto/dogi/cmd.Version=$(GIT_COMMIT)"

install: format
	@echo '- install'
	@env go install -a -ldflags="-X github.com/ntorresalberto/dogi/cmd.Version=$(GIT_COMMIT)" .

version:
	@echo '- version: ${GIT_COMMIT}'

vet:
	@echo '- go vet'
	@go vet ./...

tidy:
	@echo '- go mod tidy'
	@go mod tidy

format: version tidy vet
	@echo '- format'
	@go fmt ./...

lint:
	@echo '- lint'
	@${GOPATH}/golangci-lint run ./...

count:
	@echo '- count'
	@${GOPATH}/gocloc main.go assets/createUser.sh.in assets/assets.go cmd/

all: build lint count
