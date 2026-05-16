#!/bin/bash
# 请不要用于实际编译，本脚本仅用于Github Action。
# Please do not use this script for actual compilation, it is only for use with GitHub Actions.

sudo apt-get update
sudo apt-get install -y \
    zip \
    upx

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
go build -ldflags="-s -w" -trimpath -o 123pan-cli main.go

upx --best --lzma 123pan-cli

zip -9 123pan-cli-linux.zip 123pan-cli