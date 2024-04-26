GIT_COMMIT := $(shell git describe --always --dirty)
GOPATH=$(shell go env GOPATH)/bin
RELEASE_TAG=rolling

all: build lint count

build: format
	@echo '- build'
	@env go build -ldflags="-X github.com/ntorresalberto/dogi/cmd.Version=$(GIT_COMMIT)"

release:
	@echo '- release'
	@git tag -d ${RELEASE_TAG}
	@git push origin :refs/tags/${RELEASE_TAG}
	@git tag -f ${RELEASE_TAG}
	@git push origin rolling
	@env CGO_ENABLED=0 go build -ldflags="-X github.com/ntorresalberto/dogi/cmd.Version=$(GIT_COMMIT)"

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
