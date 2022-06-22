GIT_COMMIT := $(shell git describe --always --dirty)

build: format
	@echo '- build'
	@env CGO_ENABLED=0 go build -ldflags="-X github.com/ntorresalberto/dogi/cmd.Version=$(GIT_COMMIT)"

install: format
	@echo '- install'
	@env CGO_ENABLED=0 go install -ldflags="-X github.com/ntorresalberto/dogi/cmd.Version=$(GIT_COMMIT)" .

version:
	@echo '- version: ${GIT_COMMIT}'

tidy:
	@echo '- go mod tidy'
	@go mod tidy

format: version tidy
	@echo '- format'
	@go fmt ./...

lint:
	@echo '- lint'
	@golangci-lint run ./...
