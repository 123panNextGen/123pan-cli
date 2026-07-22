//Copyright (C) 2026 123panNextGen
//[https://github.com/123panNextGen/123pan-cli]
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"123pan-cli/internal/model"
	"123pan-cli/internal/utils"
)

// DownloadLinkInfo 下载链接信息
type DownloadLinkInfo struct {
	URL      string
	FileName string
	Size     int64
}

// GetDownloadLink 获取文件下载链接（含 URL 重写绕过流量限制）
func (c *Client) GetDownloadLink(file model.FileItem) (string, error) {
	var endpoint string
	var payload interface{}

	if file.IsDir() {
		endpoint = baseURL + "/a/api/file/batch_download_info"
		payload = map[string]interface{}{
			"fileIdList": []map[string]int64{{"fileId": file.FileID}},
		}
	} else {
		endpoint = baseURL + "/a/api/file/download_info"
		payload = map[string]interface{}{
			"driveId":   0,
			"etag":      file.Etag,
			"fileId":    file.FileID,
			"s3keyFlag": file.S3KeyFlag,
			"type":      file.Type,
			"fileName":  file.FileName,
			"size":      file.Size,
		}
	}

	var result model.DownloadInfoResponse
	if err := c.session.PostJSON(endpoint, payload, &result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		// 5113/5114: 下载流量超出限制，继续尝试 URL 重写绕过
		if result.Code != 5113 && result.Code != 5114 {
			return "", fmt.Errorf("获取下载链接失败: %s (code=%d)", result.Message, result.Code)
		}
	}

	// 优先使用 CDN 直链 (redirect_url)
	directURL := result.Data.ResolvedDownloadURL()
	if directURL != "" {
		return directURL, nil
	}

	// 使用 DownloadUrl + web-pro2 代理重写绕过限制
	downloadURL := result.Data.DownloadURL
	if downloadURL == "" {
		return "", fmt.Errorf("响应中未找到下载链接")
	}

	rewrittenURL := rewriteDownloadURL(downloadURL)
	return resolveDownloadURL(rewrittenURL)
}

// ===================== 下载 URL 重写与解析（绕过流量限制） =====================

// rewriteDownloadURL 重写下载 URL，模拟 123pan_unlock.js 的绕过逻辑。
// 将下载请求重定向到 web-pro2 代理，并添加 auto_redirect=0 参数，
// 绕过官方 PC 端的下载流量限制。
func rewriteDownloadURL(rawURL string) string {
	parsed, err := parseURL(rawURL)
	if err != nil {
		return rawURL
	}

	if strings.Contains(parsed.Host, "web-pro") {
		// 已经是 web-pro 域名，解码 params → 添加 auto_redirect → 重新编码
		qs := parseQuery(parsed.RawQuery)
		paramsB64 := qs["params"]
		if paramsB64 != "" {
			decoded := b64Decode(paramsB64)
			if decoded == "" {
				decoded = paramsB64
			}
			innerParsed, innerErr := parseURL(decoded)
			if innerErr == nil {
				innerQS := parseQuery(innerParsed.RawQuery)
				innerQS["auto_redirect"] = "0"
				innerParsed.RawQuery = buildQuery(innerQS)
				qs["params"] = b64Encode(innerParsed.String())
				parsed.RawQuery = buildQuery(qs)
				return parsed.String()
			}
		}
		return rawURL
	}

	// 非 web-pro 域名，重写为 web-pro2 代理
	origParsed, err := parseURL(rawURL)
	if err != nil {
		return rawURL
	}
	origQS := parseQuery(origParsed.RawQuery)
	origQS["auto_redirect"] = "0"
	origParsed.RawQuery = buildQuery(origQS)

	proxyURL := fmt.Sprintf(
		"https://web-pro2.123952.com/download-v2/?params=%s&is_s3=0",
		b64Encode(origParsed.String()),
	)
	return proxyURL
}

// resolveDownloadURL 解析重定向获取真实下载链接。
// 优先级：HTTP 3xx Location 头 → HTML body href 链接 → download-v2 base64 params 解码
func resolveDownloadURL(rawURL string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不自动跟随重定向
		},
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return rawURL, nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		// 网络错误时尝试直接解码 download-v2 URL 中的 base64 params
		if decoded := decodeDownloadV2Params(rawURL); decoded != "" {
			return decoded, nil
		}
		return rawURL, nil
	}
	defer resp.Body.Close()

	// 1. 优先检查 HTTP 重定向 Location 头
	if loc := resp.Header.Get("Location"); loc != "" &&
		(resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 ||
			resp.StatusCode == 307 || resp.StatusCode == 308) {
		return loc, nil
	}

	// 2. 检查 HTML body 中的 href 链接
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
	txt := string(b)
	re := regexp.MustCompile(`href='(https?://[^']+)'`)
	if m := re.FindStringSubmatch(txt); len(m) >= 2 {
		return m[1], nil
	}

	// 3. 兜底：如果是 download-v2 URL，直接解码 base64 params
	if decoded := decodeDownloadV2Params(rawURL); decoded != "" {
		return decoded, nil
	}

	return rawURL, nil
}

// decodeDownloadV2Params 从 download-v2 URL 中解码 base64 编码的下载链接。
// 格式: https://web-pro2.123952.com/download-v2/?params=<base64>&is_s3=0
func decodeDownloadV2Params(rawURL string) string {
	parsed, err := parseURL(rawURL)
	if err != nil {
		return ""
	}
	if !strings.Contains(parsed.Path, "/download-v2/") {
		return ""
	}
	qs := parseQuery(parsed.RawQuery)
	paramsB64 := qs["params"]
	if paramsB64 == "" {
		return ""
	}
	decoded := b64Decode(paramsB64)
	if strings.HasPrefix(decoded, "http") {
		return decoded
	}
	return ""
}

// checkJSONRedirect 检测响应是否为 CDN JSON 重定向，若是则返回 redirect_url。
// CDN 有时返回 JSON 而非文件内容: {"code":0,"data":{"redirect_url":"https://..."}}
func checkJSONRedirect(contentType string, body []byte) string {
	if !strings.Contains(contentType, "json") {
		return ""
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			RedirectUrl string `json:"RedirectUrl"`
			RedirectURL string `json:"redirect_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	if result.Code != 0 {
		return ""
	}
	if result.Data.RedirectUrl != "" {
		return result.Data.RedirectUrl
	}
	return result.Data.RedirectURL
}

// ===================== Base64 编解码（URL-safe） =====================

func b64Decode(data string) string {
	if data == "" {
		return ""
	}
	if decoded, err := utils.Base64Decode(data); err == nil {
		return decoded
	}
	if decoded, err := utils.Base64URLDecode(data); err == nil {
		return decoded
	}
	return data
}

func b64Encode(data string) string {
	return utils.Base64URLEncode(data)
}

// ===================== URL 解析辅助 =====================

type simpleURL struct {
	Scheme   string
	Host     string
	Path     string
	RawQuery string
}

func (u *simpleURL) String() string {
	s := u.Scheme + "://" + u.Host + u.Path
	if u.RawQuery != "" {
		s += "?" + u.RawQuery
	}
	return s
}

func parseURL(raw string) (*simpleURL, error) {
	// 简单的手动解析，避免引入 net/url 的复杂行为
	u := &simpleURL{}

	// Scheme
	schemeEnd := strings.Index(raw, "://")
	if schemeEnd < 0 {
		return nil, fmt.Errorf("invalid URL: %s", raw)
	}
	u.Scheme = raw[:schemeEnd]
	rest := raw[schemeEnd+3:]

	// Host + Path + Query
	slashIdx := strings.IndexByte(rest, '/')
	if slashIdx < 0 {
		u.Host = rest
		u.Path = "/"
	} else {
		u.Host = rest[:slashIdx]
		pathAndQuery := rest[slashIdx:]
		qIdx := strings.IndexByte(pathAndQuery, '?')
		if qIdx < 0 {
			u.Path = pathAndQuery
		} else {
			u.Path = pathAndQuery[:qIdx]
			u.RawQuery = pathAndQuery[qIdx+1:]
		}
	}
	return u, nil
}

func parseQuery(query string) map[string]string {
	result := make(map[string]string)
	if query == "" {
		return result
	}
	for _, pair := range strings.Split(query, "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func buildQuery(qs map[string]string) string {
	parts := make([]string, 0, len(qs))
	for k, v := range qs {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "&")
}

// DownloadFile 单线程下载文件（含 JSON 重定向处理和连接重试）
func (c *Client) DownloadFile(downloadURL, filename, dir string) (string, error) {
	return c.downloadSingle(downloadURL, filename, dir, nil, nil, 0)
}

// downloadSingle 单线程流式下载（内部实现，支持 JSON 重定向和重试）
func (c *Client) downloadSingle(
	downloadURL, filename, dir string,
	callback func(downloaded, total int64),
	task *model.TransferTask,
	redirectCount int,
) (string, error) {
	if redirectCount >= 3 {
		return "", fmt.Errorf("JSON 重定向次数过多，放弃下载: %s", filename)
	}
	if dir == "" {
		dir = "downloads"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(dir, filename)
	tmpPath := outPath + ".123pan.tmp"

	if _, err := os.Stat(outPath); err == nil {
		return "", fmt.Errorf("文件已存在: %s", outPath)
	}

	// 连接级错误重试：最多 3 次
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest("GET", downloadURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := c.session.Transfer().Do(req)
		if err != nil {
			if attempt < maxRetries-1 {
				wait := time.Duration(attempt+1) * 2 * time.Second
				time.Sleep(wait)
				os.Remove(tmpPath)
				continue
			}
			return "", err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return "", fmt.Errorf("下载失败 HTTP %d: %s", resp.StatusCode, filename)
		}

		// 检测 JSON 重定向响应
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "json") {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			redirectURL := checkJSONRedirect(contentType, body)
			if redirectURL != "" && strings.HasPrefix(redirectURL, "http") {
				return c.downloadSingle(redirectURL, filename, dir, callback, task, redirectCount+1)
			}
			// JSON 但不是有效重定向
			return "", fmt.Errorf("下载 %s 失败，CDN 返回: %s", filename, string(body))
		}

		total := resp.ContentLength
		var downloaded int64

		f, err := os.Create(tmpPath)
		if err != nil {
			resp.Body.Close()
			return "", err
		}

		buf := make([]byte, 32*1024)
		readErr := error(nil)
		for readErr == nil {
			if task != nil {
				select {
				case <-task.Cancel:
					f.Close()
					resp.Body.Close()
					os.Remove(tmpPath)
					return "", fmt.Errorf("下载已取消")
				case <-task.Pause:
					<-task.Resume
				default:
				}
			}

			n, err := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := f.Write(buf[:n]); writeErr != nil {
					f.Close()
					resp.Body.Close()
					os.Remove(tmpPath)
					return "", writeErr
				}
				downloaded += int64(n)
				if callback != nil {
					callback(downloaded, total)
				}
				if total > 0 && task != nil {
					task.Progress = int(downloaded * 100 / total)
				}
			}
			readErr = err
		}
		resp.Body.Close()
		f.Close()

		if readErr != io.EOF {
			os.Remove(tmpPath)
			if attempt < maxRetries-1 {
				wait := time.Duration(attempt+1) * 2 * time.Second
				time.Sleep(wait)
				continue
			}
			return "", readErr
		}

		if err := os.Rename(tmpPath, outPath); err != nil {
			os.Remove(tmpPath)
			return "", err
		}
		return outPath, nil
	}
	return "", fmt.Errorf("下载失败：超过最大重试次数")
}

// DownloadFileWithProgress 下载文件（带进度回调和任务控制）
func (c *Client) DownloadFileWithProgress(
	downloadURL, filename, dir string,
	callback func(downloaded, total int64),
	task *model.TransferTask,
) (string, error) {
	return c.downloadSingle(downloadURL, filename, dir, callback, task, 0)
}

// DownloadFileChunked 多线程分片下载
func (c *Client) DownloadFileChunked(
	downloadURL, filename, dir string,
	fileSize int64, numThreads int,
	callback func(downloaded, total int64),
	task *model.TransferTask,
) (string, error) {
	if dir == "" {
		dir = "downloads"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	outPath := filepath.Join(dir, filename)
	tmpPath := outPath + ".123pan.tmp"

	if _, err := os.Stat(outPath); err == nil {
		return "", fmt.Errorf("文件已存在: %s", outPath)
	}

	// 检查服务器是否支持 Range 请求
	headReq, _ := http.NewRequest("HEAD", downloadURL, nil)
	headReq.Header.Set("User-Agent", "Mozilla/5.0")
	headResp, err := c.session.Transfer().Do(headReq)
	supportsRange := false
	if err == nil {
		supportsRange = headResp.Header.Get("Accept-Ranges") == "bytes"
		headResp.Body.Close()
	}

	if !supportsRange || fileSize < 5*1024*1024 || numThreads <= 1 {
		return c.DownloadFileWithProgress(downloadURL, filename, dir, callback, task)
	}

	// 分片下载
	partSize := fileSize / int64(numThreads)
	var downloaded int64
	var mu sync.Mutex
	partPaths := make([]string, numThreads)

	var wg sync.WaitGroup
	errCh := make(chan error, numThreads)

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			start := int64(idx) * partSize
			end := start + partSize - 1
			if idx == numThreads-1 {
				end = fileSize - 1
			}

			partPath := fmt.Sprintf("%s.part%d", tmpPath, idx)
			partPaths[idx] = partPath

			req, _ := http.NewRequest("GET", downloadURL, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0")
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

			resp, err := c.session.Transfer().Do(req)
			if err != nil {
				errCh <- err
				return
			}
			defer resp.Body.Close()

			f, err := os.Create(partPath)
			if err != nil {
				errCh <- err
				return
			}
			defer f.Close()

			buf := make([]byte, 32*1024)
			for {
				select {
				case <-task.Cancel:
					return
				case <-task.Pause:
					<-task.Resume
				default:
				}

				n, readErr := resp.Body.Read(buf)
				if n > 0 {
					f.Write(buf[:n])
					mu.Lock()
					downloaded += int64(n)
					current := downloaded
					mu.Unlock()
					if callback != nil {
						callback(current, fileSize)
					}
					if task != nil && fileSize > 0 {
						task.Progress = int(current * 100 / fileSize)
					}
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					errCh <- readErr
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	if err, ok := <-errCh; ok {
		// 清理分片
		for _, p := range partPaths {
			os.Remove(p)
		}
		return "", err
	}

	// 检查取消
	select {
	case <-task.Cancel:
		for _, p := range partPaths {
			os.Remove(p)
		}
		return "", fmt.Errorf("下载已取消")
	default:
	}

	// 合并分片
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}

	for _, p := range partPaths {
		f, err := os.Open(p)
		if err != nil {
			out.Close()
			return "", err
		}
		io.Copy(out, f)
		f.Close()
		os.Remove(p)
	}
	out.Close()

	if err := os.Rename(tmpPath, outPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return outPath, nil
}

// DownloadFileByIndex 按索引下载文件
func (c *Client) DownloadFileByIndex(index int, dir string) (string, error) {
	if index < 1 || index > len(c.fileList) {
		return "", fmt.Errorf("索引 %d 超出范围", index)
	}
	file := c.fileList[index-1]

	url, err := c.GetDownloadLink(file)
	if err != nil {
		return "", err
	}

	filename := file.FileName
	if file.IsDir() {
		filename += ".zip"
	}

	return c.DownloadFile(url, filename, dir)
}

// DownloadDir 递归下载目录
func (c *Client) DownloadDir(file model.FileItem, downloadPathRoot string) error {
	if !file.IsDir() {
		return fmt.Errorf("不是文件夹")
	}

	allFiles, err := c.ListAllFiles(file.FileID)
	if err != nil {
		return err
	}

	for i := len(allFiles) - 1; i >= 0; i-- {
		f := allFiles[i]
		if f.IsDir() {
			continue
		}
		url, err := c.GetDownloadLink(f)
		if err != nil {
			return fmt.Errorf("获取 %s 下载链接失败: %w", f.FileName, err)
		}
		downloadPath := filepath.Join(downloadPathRoot, file.FileName)
		if _, err := c.DownloadFile(url, f.FileName, downloadPath); err != nil {
			return fmt.Errorf("下载 %s 失败: %w", f.FileName, err)
		}
	}
	return nil
}
