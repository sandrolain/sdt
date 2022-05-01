#!/bin/sh

GOOS=js GOARCH=wasm go build -o "./bin/sdt.wasm" "./app"

cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" "./bin/"
