#!/bin/sh

#tinygo build -o "./web/sdt.wasm" -target wasm "./cli/main.go"

GOOS=js GOARCH=wasm go build -o "./web/src/sdt.wasm" "./cli"

cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" "./web/src/"
