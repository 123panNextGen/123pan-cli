//Copyright (C) 2026 123panNextGen
//[https://github.com/123panNextGen/123pan-cli]
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package utils

import "fmt"

// FormatFileSize 格式化文件大小
func FormatFileSize(size int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	fSize := float64(size)
	unitIndex := 0
	for fSize >= 1024.0 && unitIndex < len(units)-1 {
		fSize /= 1024.0
		unitIndex++
	}
	return fmt.Sprintf("%.2f %s", fSize, units[unitIndex])
}

// TruncateString 截断字符串
func TruncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

// PadRight 右填充
func PadRight(s string, length int) string {
	runes := []rune(s)
	if len(runes) >= length {
		return s
	}
	return s + fmt.Sprintf("%*s", length-len(runes), "")
}
