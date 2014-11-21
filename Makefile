#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2014-11-21 16:44:10 +0000 (Fri, 21 Nov 2014)
#
#  vim:ts=2:sw=2:et
#

NAME=embassy
AUTHOR=gambol99
HARDWARE=$(shell uname -m)
VERSION=0.0.1

build:
	go build -o stage/embassy
	docker build -t ${AUTHOR}/${NAME} .

clean:
	rm -f ./stage/embassy
	go clean

release:
	rm -rf release
	mkdir release
	GOOS=linux go build -o release/$(NAME)
	cd release && tar -zcf $(NAME)_$(VERSION)_linux_$(HARDWARE).tgz $(NAME)
	GOOS=darwin go build -o release/$(NAME)
	cd release && tar -zcf $(NAME)_$(VERSION)_darwin_$(HARDWARE).tgz $(NAME)
	rm release/$(NAME)
	echo "$(VERSION)" > release/version
	echo "progrium/$(NAME)" > release/repo

.PHONY: build
