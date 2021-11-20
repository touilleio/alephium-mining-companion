
VERSION=v5.1.0
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOLINT=golangci-lint run
BUILD_PLATFORM=linux/amd64
PACKAGE_PLATFORM=$(BUILD_PLATFORM),linux/arm64,linux/arm/v7
#PACKAGE_PLATFORM=$(BUILD_PLATFORM)
VERSION_MAJOR=$(shell echo $(VERSION) | cut -f1 -d.)
VERSION_MINOR=$(shell echo $(VERSION) | cut -f2 -d.)
BINARY_NAME=alephium-mining-companion
GO_PACKAGE=touilleio/alephium-mining-companion
DOCKER_REGISTRY=
GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_DIRTY=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)
BUILD_DATE=$(shell date '+%Y-%m-%d-%H:%M:%S')

all: ensure build package

ensure:
	$(GOCMD) mod download

clean:
	$(GOCLEAN)

lint:
	$(GOLINT) ...

build:
	env CGO_ENABLED=0 $(GOCMD) mod download && \
	env CGO_ENABLED=0 \
		$(GOBUILD) \
		-ldflags "-X github.com/sqooba/go-common/version.GitCommit=${GIT_COMMIT}${GIT_DIRTY} \
			-X github.com/sqooba/go-common/version.BuildDate=${BUILD_DATE} \
			-X github.com/sqooba/go-common/version.Version=${VERSION}" \
		-o $(BINARY_NAME) \
		.

package:
	docker buildx build -f Dockerfile \
		--platform $(BUILD_PLATFORM) \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION_MAJOR).$(VERSION_MINOR) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION_MAJOR) \
		--load --no-cache \
		.

test:
	go test ./...

release:
	docker buildx build -f Dockerfile \
		--platform $(PACKAGE_PLATFORM) \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION_MAJOR).$(VERSION_MINOR) \
		-t ${DOCKER_REGISTRY}${GO_PACKAGE}:$(VERSION_MAJOR) \
		--push \
		.
