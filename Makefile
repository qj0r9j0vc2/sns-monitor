# Makefile for sns-monitor project

include .env

APP_NAME=sns-monitor
PLATFORM=linux/amd64
DOCKER_TAG?=latest
LOG_LEVEL?=debug

BIN_DIR=bin

.PHONY: all build build-client build-server build-lambda docker docker-buildx run-docker docker-lambda docker-lambda-buildx clean env

all: build

build:
	go build -o $(BIN_DIR)/$(APP_NAME) ./cmd

build-client:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_DIR)/$(APP_NAME)-client ./cmd
	@echo "✅ Built client binary"

build-server:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_DIR)/$(APP_NAME)-server ./cmd
	@echo "✅ Built server binary"

build-lambda:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap ./cmd
	@echo "✅ Lambda bootstrap built"

# Docker for normal service (client/server)
docker:
	docker build -f Dockerfile -t $(APP_NAME):$(DOCKER_TAG) .

run-docker:
	docker run --rm -e MODE=$(MODE) --env-file .env -p 8080:8080 $(APP_NAME):$(DOCKER_TAG)

# Docker for AWS Lambda container image
docker-lambda:
	docker build -f Dockerfile.lambda -t $(APP_NAME)-lambda:$(DOCKER_TAG) .

# BuildKit builds
docker-buildx:
	docker buildx build --platform=$(PLATFORM) -t $(APP_NAME):$(DOCKER_TAG) --provenance=false --load -f Dockerfile .

docker-lambda-buildx:
	docker buildx build --platform=$(PLATFORM) -t $(APP_NAME)-lambda:$(DOCKER_TAG) --provenance=false --load -f Dockerfile.lambda .

# Clean build artifacts
clean:
	rm -rf $(BIN_DIR) bootstrap lambda.zip

# Load .env
env:
	@if [ -f .env ]; then \
		echo "Loading environment variables from .env..."; \
		export $$(cat .env | xargs); \
	fi;
