#!/bin/sh

go test ./app/cmd -v -coverprofile cover.out && go tool cover -html=cover.out
