#!/bin/bash
set -e

echo "=== 123pan-cli Build Script ==="

# 安装依赖（CI 环境）
if [ -n "$CI" ]; then
    sudo apt-get update -qq
    sudo apt-get install -y -qq zip upx
fi

# 编译
echo ">> Building linux amd64..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
go build -ldflags="-s -w" -trimpath -o 123pan-cli ./cmd/123pan-cli/

# 压缩
echo ">> Compressing with upx..."
upx --best --lzma 123pan-cli

# 打包
echo ">> Creating archive..."
zip -9 123pan-cli-linux.zip 123pan-cli

echo ">> Build complete: 123pan-cli-linux.zip"
