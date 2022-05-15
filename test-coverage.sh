#!/bin/sh

go test ./cli/cmd -v -coverprofile cover.out && go tool cover -html=cover.out
