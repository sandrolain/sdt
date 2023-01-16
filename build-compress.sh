#!/bin/sh

DEST="./bin/sdt"

go build -o $DEST "./cli"

upx $DEST
