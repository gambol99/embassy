#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-21 16:44:10 +0000 (Fri, 21 Nov 2014)
#
#
NAME=embassy
AUTHOR=gambol99
HARDWARE=$(shell uname -m)
VERSION=$(shell awk '/VERSION/ { print $$3 }' version.go | sed 's/"//g')
REPOSITORY=embassy
DEPS=$(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
PACKAGES=$(shell go list ./...)
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods -nilfunc -printf -rangeloops -shift -structtags -unsafeptr


.PHONY: build docker release

build:
	go get github.com/tools/godep
	godep go build -o bin/embassy

static:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w' -o bin/${NAME}

docker: static
	sudo docker build -t ${AUTHOR}/${NAME} .

all: clean build docker

vet:
	@echo "--> Running go vet $(VETARGS) ."
	@go tool vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		go get golang.org/x/tools/cmd/vet; \
	fi
	@go tool vet $(VETARGS) *.go

lint:
	@echo "--> Running golint"
	@which golint 2>/dev/null ; if [ $$? -eq 1 ]; then \
		go get -u github.com/golang/lint/golint; \
	fi
	@golint $(PACKAGES)

format:
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

clean:
	rm -f ./bin/embassy
	rm -rf ./release
	go clean

changelog: release
	git log $(shell git tag | tail -n1)..HEAD --no-merges --format=%B > changelog

release: clean build changelog
	rm -rf release
	mkdir -p release
	GOOS=linux godep go build -o release/$(NAME)
	cd release && gzip -c embassy > $(NAME)_$(VERSION)_linux_$(HARDWARE).gz
	GOOS=darwin godep go build -o release/$(NAME)
	cd release && gzip -c embassy > $(NAME)_$(VERSION)_darwin_$(HARDWARE).gz
	rm release/$(NAME)
