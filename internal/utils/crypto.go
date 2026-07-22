//Copyright (C) 2026 123panNextGen
//[https://github.com/123panNextGen/123pan-cli]
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package utils

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
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

// Base64Decode 标准 base64 解码
func Base64Decode(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Base64URLDecode URL-safe base64 解码（无填充）
func Base64URLDecode(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		// 尝试带填充
		b, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return "", err
		}
	}
	return string(b), nil
}

// Base64URLEncode URL-safe base64 编码（无填充）
func Base64URLEncode(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}
