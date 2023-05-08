# build params
PREFIX?=registry.aliyuncs.com/ringtail
VERSION?=v0.0.1
GIT_COMMIT:=$(shell git rev-parse --short HEAD)

# Image URL to use all building/pushing image targets
IMG ?= $(PREFIX)/kubernetes-webhook-injector:$(VERSION)-$(GIT_COMMIT)-aliyun
PLUGIN_IMG ?= $(PREFIX)/webhook-injector-buildin-plugins:$(VERSION)-$(GIT_COMMIT)-aliyun
all: test build-binary

# Run tests
test: fmt vet
	go test ./pkg/... ./plugins/...  -coverprofile cover.out

# Build kubernetes-webhook-injector binary
build-binary:
	go build -o bin/kubernetes-faketime-injector main.go

build-plugins:
	go build -o bin/security-group-plugin ./plugins/security_group/cmd
	go build -o bin/rds-whitelist-plugin ./plugins/rds_whitelist/cmd
	go build -o bin/redis-whitelist-plugin ./plugins/redis_whitelist/cmd
	go build -o bin/slb-access-control-plugin ./plugins/slb_access_control_policy/cmd


# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt vet
	go run ./main.go

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./plugins/...

# Run go vet against code
vet:
	go vet ./pkg/... ./plugins/...

# Build the docker image
docker-build:
	docker build . -f build/Dockerfile -t ${IMG} --no-cache
	docker build . -f build/Dockerfile_Plugins -t ${PLUGIN_IMG} --no-cache

# Push the docker image
docker-push:
	docker push ${IMG}
	docker push ${PLUGIN_IMG}
