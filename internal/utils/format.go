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
