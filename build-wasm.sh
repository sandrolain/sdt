#!/bin/sh

GOOS=js GOARCH=wasm go build -o "./web/sdt.wasm" "./cli"

cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" "./web/"
