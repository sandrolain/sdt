#!/bin/sh

tag=`git tag --sort=-version:refname | head -n 1`-`git show -s --format=%ct`

docker build -t sandrolain/sdt:$tag -t sandrolain/sdt:latest .
