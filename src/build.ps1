GOOS=windows GOARCH=amd64 CGO_ENABLED=0 `
go build -ldflags="-s -w" -trimpath -o 123pan-cli.exe main.go