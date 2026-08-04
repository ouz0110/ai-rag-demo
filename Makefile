api=api/ai-rag-demo
PROJECT=ai-rag-demo-api

GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --abbrev=0 --tags)
BUILD_TIME = $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT_HASH = $(shell git rev-parse --verify HEAD)
INTERNAL_PROTO_FILES=$(shell find internal -name *.proto)
API_PROTO_FILES=$(shell find $(api) -name *.proto)
VALIDATE_PROTO_FILES=$(shell find $(api) -name *.proto | grep -v error)
SERVICE=$(shell ls cmd | head -1)

.PHONY: init
# init env
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	go install github.com/go-kratos/kratos/cmd/kratos/v2@v2.0.0-20240105030612-34d9666e0e1b
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@v2.0.0-20240105030612-34d9666e0e1b
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@v2.0.0-20240105030612-34d9666e0e1b
	go install github.com/google/wire/cmd/wire@v0.7.0
	go install github.com/envoyproxy/protoc-gen-validate@v1.0.2
	go install github.com/favadi/protoc-go-inject-tag@v1.4.0

.PHONY: build
# build
build:
	go mod tidy
	mkdir -p bin/ && go build -trimpath -ldflags "-s -w -X 'main.Version=$(VERSION)' -X 'main.Revision=$(GIT_COMMIT_HASH)' -X 'main.BuildTime=$(BUILD_TIME)'" -o ./bin/server ./cmd/server/

.PHONY: pack
# build linux binary, frontend, copy configs and docker-compose to build/api-rag-demo
pack:
	go mod tidy
	rm -rf build/api-rag-demo
	mkdir -p build/api-rag-demo/web
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X 'main.Version=$(VERSION)' -X 'main.Revision=$(GIT_COMMIT_HASH)' -X 'main.BuildTime=$(BUILD_TIME)'" -o ./build/api-rag-demo/api-rag-demo-server ./cmd/server/
	cp -r configs ./build/api-rag-demo/
	sed -i.bak -e 's/localhost:6379/redis:6379/g' -e 's/localhost:3306/mysql:3306/g' -e 's/localhost:19530/standalone:19530/g' ./build/api-rag-demo/configs/config.local.yaml && rm -f ./build/api-rag-demo/configs/config.local.yaml.bak
	cd web && npm run build
	cp -r web/dist/* ./build/api-rag-demo/web/
	cp docker-compose.yml ./build/api-rag-demo/


.PHONY: generate
# generate
generate:
	go mod tidy
	go get github.com/google/wire/cmd/wire@v0.7.0
	GOWORK=off go generate ./...

.PHONY: wire
# generate wire
wire:
	go mod tidy;
	cd cmd/$(SERVICE) && wire

.PHONY: gotags
gotags:
	protoc-go-inject-tag -input="./api/*/v1/*.go"



.PHONY: all
# generate all
all:
	go mod tidy;
	make api;
	make gotags;
	make wire;
	make generate;
	make build;

.PHONY: api
# generate api
api:
	find api -name "*.go" | xargs rm -f
	find api -name "*.go-e" | xargs rm -f
	go run api.go


.PHONY: extract18n
extract18n:
	goi18n extract -sourceLanguage=zh-cn

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

.PHONY: dev
dev:
	go run cmd/server/main.go -conf=configs/config.yaml

