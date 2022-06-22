GIT_COMMIT := $(shell git describe --always --dirty)

install: format
	@echo '- install'
	@env CGO_ENABLED=0 go install -ldflags="-X github.com/ntorresalberto/dogi/cmd.Version=$(GIT_COMMIT)" .

version:
	@echo '- version: ${GIT_COMMIT}'

format: version
	@echo '- format'
	@go fmt ./...

build: version
	@echo '- build'
	@env CGO_ENABLED=0 go build -ldflags="-X github.com/ntorresalberto/dogi/cmd.Version=$(GIT_COMMIT)"

lint:
	@echo '- lint'
	@golangci-lint run ./...
