#!/bin/sh

GOOS=js GOARCH=wasm go build -o "./src/sdt.wasm" "../cli"
