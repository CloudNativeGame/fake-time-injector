# build params
VERSION?=v0.0.1
GIT_COMMIT:=$(shell git rev-parse --short HEAD)

# Image URL to use all building/pushing image targets
IMG ?= fake-time-injector:$(VERSION)-$(GIT_COMMIT)
all: test build-binary

# Run tests
test: fmt vet
	go test ./pkg/... -coverprofile cover.out

# Build kubernetes-webhook-injector binary
build-binary:
	go build -o bin/fake-time-injector main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt vet
	go run ./main.go

# Run go fmt against code
fmt:
	go fmt ./pkg/...

# Run go vet against code
vet:
	go vet ./pkg/...

# Build the docker image
docker-build:
	docker build . -f Dockerfile -t ${IMG} --no-cache

