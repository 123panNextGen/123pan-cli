$ErrorActionPreference = "Stop"

Write-Host "=== 123pan-cli Build Script ==="

# 安装 upx（CI 环境）
if ($env:CI) {
    choco install upx -y --no-progress
}

# 编译
Write-Host ">> Building windows amd64..."
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -trimpath -o 123pan-cli.exe ./cmd/123pan-cli/

# 压缩
Write-Host ">> Compressing with upx..."
upx --best --lzma 123pan-cli.exe

# 打包
Write-Host ">> Creating archive..."
Compress-Archive `
    -Path 123pan-cli.exe `
    -DestinationPath 123pan-cli-windows.zip `
    -Force

Write-Host ">> Build complete: 123pan-cli-windows.zip"
