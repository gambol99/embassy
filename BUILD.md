
#### **Building**

Assuming the following GO environment
  
    [jest@starfury embassy]$ set | grep GO
    GOPATH=/home/jest/go
    GOROOT=/usr/lib/golang
    
    cd $GOPATH
    mkdir -p src/github.com/gambol99
    cd src/github.com/gambol99
    git clone https://github.com/gambol99/embassy.git
    cd embassy && make

An alternative would be to build inside a golang container
  
    cd /tmp
    git clone https://github.com/gambol99/embassy.git
    cd embassy
    docker run --rm -v "$PWD":/go/src/github.com/gambol99/embassy \
      -w /go/src/github.com/gambol99/embassy -e GOOS=linux golang:1.3.3 make
    stage/embassy --help

