#!/bin/sh

cd ./web
npm run build

cp -R ./dist ../web-server/dist

cd ../web-server

GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o linux/amd64/sdtserve
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o linux/arm64/sdtserve

upx --best --lzma linux/amd64/sdtserve
upx --best --lzma linux/arm64/sdtserve

docker rmi -f sandrolain/sdt:latest
docker buildx build -t sandrolain/sdt:latest --platform=linux/amd64,linux/arm64 . --push
