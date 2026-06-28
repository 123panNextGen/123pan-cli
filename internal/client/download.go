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
	"sync/atomic"
	"time"

	"123pan-cli/internal/model"
)

// DownloadLinkInfo 下载链接信息
type DownloadLinkInfo struct {
	URL      string
	FileName string
	Size     int64
}

// progressReader 包装 io.Reader，支持进度回调
type progressReader struct {
	reader   io.Reader
	total    int64
	read     *int64
	callback func(downloaded, total int64)
	limiter  chan struct{} // 简单的速率限制
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		atomic.AddInt64(pr.read, int64(n))
		if pr.callback != nil {
			pr.callback(atomic.LoadInt64(pr.read), pr.total)
		}
	}
	return n, err
}

// GetDownloadLink 获取文件下载链接
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

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := c.session.Do("POST", endpoint, jsonData)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			DownloadURL string `json:"DownloadUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析下载链接响应失败: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("获取下载链接失败: %s", result.Message)
	}
	if result.Data.DownloadURL == "" {
		return "", fmt.Errorf("空的下载链接")
	}

	return c.resolveDownloadURL(result.Data.DownloadURL)
}

// resolveDownloadURL 解析下载 URL 的重定向
func (c *Client) resolveDownloadURL(rawURL string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if loc := resp.Header.Get("Location"); loc != "" {
		return loc, nil
	}

	b, _ := io.ReadAll(resp.Body)
	txt := string(b)

	// 尝试从响应体中提取 URL
	re := regexp.MustCompile(`href='(https?://[^']+)'`)
	if m := re.FindStringSubmatch(txt); len(m) >= 2 {
		return m[1], nil
	}
	re = regexp.MustCompile(`href="(https?://[^"]+)"`)
	if m := re.FindStringSubmatch(txt); len(m) >= 2 {
		return m[1], nil
	}

	return rawURL, nil
}

// DownloadFile 单线程下载文件
func (c *Client) DownloadFile(downloadURL, filename, dir string) (string, error) {
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

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := c.session.Transfer().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("下载失败 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", err
	}
	f.Close()

	if err := os.Rename(tmpPath, outPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return outPath, nil
}

// DownloadFileWithProgress 下载文件（带进度回调）
func (c *Client) DownloadFileWithProgress(
	downloadURL, filename, dir string,
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

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := c.session.Transfer().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("下载失败 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	total := resp.ContentLength
	var downloaded int64

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-task.Cancel:
			os.Remove(tmpPath)
			return "", fmt.Errorf("下载已取消")
		case <-task.Pause:
			<-task.Resume
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
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
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			os.Remove(tmpPath)
			return "", readErr
		}
	}

	if err := os.Rename(tmpPath, outPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return outPath, nil
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
