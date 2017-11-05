.SILENT :

# App name
APPNAME=kong-docker-daemon

# Go configuration
GOOS?=linux
GOARCH?=amd64

# Add exe extension if windows target
is_windows:=$(filter windows,$(GOOS))
EXT:=$(if $(is_windows),".exe","")

# Go app path
APP_BASE=${GOPATH}/src/github.com/ncarlier

# Artefact name
ARTEFACT=release/$(APPNAME)-$(GOOS)-$(GOARCH)$(EXT)

# Extract version infos
VERSION:=`git describe --tags`
LDFLAGS=-ldflags "-X github.com/ncarlier/${APPNAME}/main.Version=${VERSION}"

all: build

# Include common Make tasks
root_dir:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
makefiles:=$(root_dir)/makefiles
include $(makefiles)/help.Makefile
include $(makefiles)/docker/compose.Makefile

$(APP_BASE)/$(APPNAME):
	echo "Creating GO src link: $(APP_BASE)/$(APPNAME) ..."
	mkdir -p $(APP_BASE)
	ln -s $(root_dir) $(APP_BASE)/$(APPNAME)

glide.lock:
	echo "Installing dependencies ..."
	glide install

## Clean built files
clean:
	-rm -rf release
.PHONY : clean

## Build executable
build: glide.lock $(APP_BASE)/$(APPNAME)
	mkdir -p release
	echo "Building: $(ARTEFACT) ..."
	cd $(APP_BASE)/$(APPNAME) && GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(ARTEFACT)
.PHONY : build

$(ARTEFACT): build

## Install executable
install: $(ARTEFACT)
	echo "Installing $(ARTEFACT) to ${HOME}/.local/bin/$(APPNAME) ..."
	cp $(ARTEFACT) ${HOME}/.local/bin/$(APPNAME)
.PHONY : install

## Deploy containers to Docker host
deploy: compose-up
.PHONY : deploy

## Un-deploy API from Docker host
undeploy: compose-down
.PHONY : undeploy
