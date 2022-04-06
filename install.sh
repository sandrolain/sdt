#!/bin/sh

path=`go env GOPATH`
go build -o "$path/bin/sdt" "./app"
