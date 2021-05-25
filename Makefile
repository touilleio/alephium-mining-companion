
VERSION=v1.0.0
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOLINT=golangci-lint run
VERSION_MAJOR=$(shell echo $(VERSION) | cut -f1 -d.)
VERSION_MINOR=$(shell echo $(VERSION) | cut -f2 -d.)
BINARY_NAME=alephium-mining-sidecar
GO_PACKAGE=touilleio/alephium-mining-sidecar
DOCKER_REGISTRY=

all: ensure build package

ensure:
	GOOS=${GOOS} $(GOCMD) mod download

clean:
	$(GOCLEAN)

lint:
	$(GOLINT) ...

build:
	env CGO_ENABLED=0 GOOS=linux go mod download && \
	export GIT_COMMIT=$(shell git rev-parse HEAD) && \
	export GIT_DIRTY=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true) && \
	export BUILD_DATE=$(shell date '+%Y-%m-%d-%H:%M:%S') && \
	env CGO_ENABLED=0 GOOS=linux \
		go build -o alephium-mining-sidecar \
		-ldflags "-X github.com/sqooba/go-common/version.GitCommit=$${GIT_COMMIT}${GIT_DIRTY} \
			-X github.com/sqooba/go-common/version.BuildDate=$${BUILD_DATE} \
			-X github.com/sqooba/go-common/version.Version=$${VERSION}" \
		.

package:
	docker buildx build -f Dockerfile \
		--platform linux/amd64 \
		--build-arg VERSION=$(VERSION) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION_MAJOR).$(VERSION_MINOR) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION_MAJOR) \
		--load --no-cache \
		.

test:
	go test ./...

release:
	docker buildx build -f Dockerfile \
		--platform linux/amd64,linux/arm64,linux/arm/v7 \
		--build-arg VERSION=$(VERSION) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION_MAJOR).$(VERSION_MINOR) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION_MAJOR) \
		--push \
		.
