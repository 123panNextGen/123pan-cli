# 请不要用于实际编译，本脚本仅用于Github Action。
# Please do not use this script for actual compilation, it is only for use with GitHub Actions.

choco install upx -y

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -trimpath -o 123pan-cli.exe main.go

upx --best --lzma 123pan-cli.exe

Compress-Archive `
    -Path 123pan-cli.exe `
    -DestinationPath 123pan-windows.zip `
    -Force