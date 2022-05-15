#!/bin/sh

path=`go env GOPATH`
go mod vendor
go build -ldflags "-X main.version=`git tag --sort=-version:refname | head -n 1`-custom -X main.commit=`git rev-parse HEAD` -X main.date=`date +"%Y-%m-%dT%H:%M:%S%z"`" -o "$path/bin/sdt" "./cli"
