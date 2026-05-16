#!/bin/bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
go build -ldflags="-s -w" -trimpath -o 123pan-cli main.go