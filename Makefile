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


build:
	go get github.com/tools/godep
	godep go build -o stage/embassy
	docker build -t ${AUTHOR}/${NAME} .

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
	git log $(shell git tag | tail -n1)..HEAD --no-merges --format=%B > release/changelog

github-release:
	git tag v$(VERSION)
	git push --tags
	github-release release -r $(REPOSITORY) --tag v$(VERSION) -p -d "$(shell cat release/changelog)"
	github-release upload  -r $(REPOSITORY) --tag v$(VERSION) release/$(NAME)_$(VERSION)_darwin_$(HARDWARE).gz
	github-release upload  -r $(REPOSITORY) --tag v$(VERSION) release/$(NAME)_$(VERSION)_linux_$(HARDWARE).gz

.PHONY: build release
