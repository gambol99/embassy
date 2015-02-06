#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-21 16:44:10 +0000 (Fri, 21 Nov 2014)
#
#  vim:ts=2:sw=2:et
#

NAME=embassy
AUTHOR=gambol99
HARDWARE=$(shell uname -m)
VERSION=$(shell awk '/const Version/ { print $$4 }' version.go | sed 's/"//g')
REPOSITORY=embassy

.PHONY: build docker release

build:
	go get
	go get github.com/tools/godep
	godep go build -o stage/embassy

docker: build
	docker build -t ${AUTHOR}/${NAME} .

all: clean build docker

clean:
	rm -f ./stage/embassy
	rm -rf ./release
	go clean

release:
	rm -rf release
	mkdir -p release
	GOOS=linux godep go build -o release/$(NAME)
	cd release && gzip -c embassy > $(NAME)_$(VERSION)_linux_$(HARDWARE).gz
	GOOS=darwin godep go build -o release/$(NAME)
	cd release && gzip -c embassy > $(NAME)_$(VERSION)_darwin_$(HARDWARE).gz
	rm release/$(NAME)

changelog: release
	git log $(shell git tag | tail -n1)..HEAD --no-merges --format=%B > release/changelog

