package utils

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
)

// ComputeFileMD5 计算文件的 MD5 哈希值
func ComputeFileMD5(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.CopyBuffer(h, f, make([]byte, 64*1024)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeBytesMD5 计算字节的 MD5
func ComputeBytesMD5(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// RandomHex 生成随机十六进制字符串 (crypto/rand)
func RandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// fallback
		for i := range b {
			b[i] = byte(i)
		}
	}
	return hex.EncodeToString(b)
}
