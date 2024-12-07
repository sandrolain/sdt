#!/bin/sh

GOOS=js GOARCH=wasm go build -o "./src/sdt.wasm" "../cli"

cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "./src/"
