.SILENT :
.PHONY : clean build

# App name
APPNAME=kong-docker-daemon

# Base image
BASEIMAGE=golang:1.8

# Go configuration
GOOS?=linux
GOARCH?=amd64

# Extract version infos
VERSION:=`git describe --tags`
LDFLAGS=-ldflags "-X github.com/ncarlier/${APPNAME}/main.Version=${VERSION}"

all: build

# Include common Make tasks
root_dir:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
makefiles:=$(root_dir)/makefiles
include $(makefiles)/help.Makefile
include $(makefiles)/compose.Makefile

glide.lock:
	glide install

## Clean built files
clean:
	-rm -rf release

## Build executable
build: glide.lock
	mkdir -p release
	echo "Building: release/$(APPNAME)-$(GOOS)-$(GOARCH)$(EXT) ..."
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o release/$(APPNAME)-$(GOOS)-$(GOARCH)$(EXT)

## Run executable
exec: build
	release/$(APPNAME)-$(GOOS)-$(GOARCH)$(EXT) -v

## Deploy containers to Docker host
deploy: images
	echo "Deploying infrastructure..."
	-cat .env
	docker-compose $(COMPOSE_FILES) up -d
	echo "Congrats! Infrastructure deployed."

## Un-deploy API from Docker host
undeploy:
	echo "Un-deploying infrastructure..."
	docker-compose $(COMPOSE_FILES) down
	echo "Infrastructure un-deployed."