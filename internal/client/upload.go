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
	"time"

	"123pan-cli/internal/model"
	"123pan-cli/internal/utils"
)

const blockSize = 5 * 1024 * 1024 // 5MB

// UploadFile 上传文件
func (c *Client) UploadFile(filePath string) (int64, error) {
	return c.uploadFileStream(filePath, 0, nil, nil)
}

// UploadFileWithProgress 上传文件（带进度和任务控制）
func (c *Client) UploadFileWithProgress(
	filePath string,
	dupChoice int,
	callback func(uploaded, total int64),
	task *model.TransferTask,
) (int64, error) {
	return c.uploadFileStream(filePath, dupChoice, callback, task)
}

func (c *Client) uploadFileStream(
	filePath string,
	dupChoice int,
	callback func(uploaded, total int64),
	task *model.TransferTask,
) (int64, error) {
	// 规范化路径
	filePath = filepath.Clean(filePath)
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("文件不存在: %w", err)
	}
	if info.IsDir() {
		return 0, fmt.Errorf("不支持文件夹上传")
	}

	fsize := info.Size()
	fileName := filepath.Base(filePath)

	// 计算文件 MD5
	md5Hash, err := utils.ComputeFileMD5(filePath)
	if err != nil {
		return 0, fmt.Errorf("计算MD5失败: %w", err)
	}

	// 1. 上传请求
	uploadReq := map[string]interface{}{
		"driveId":      0,
		"etag":         md5Hash,
		"fileName":     fileName,
		"parentFileId": c.currentDirID,
		"size":         fsize,
		"type":         0,
		"duplicate":    dupChoice,
	}

	var uploadResp model.UploadRequestResponse
	if err := c.session.PostJSON(baseURL+"/b/api/file/upload_request", uploadReq, &uploadResp); err != nil {
		return 0, err
	}
	if uploadResp.Code == 5060 {
		// 同名文件存在，使用 dupChoice 重试
		uploadReq["duplicate"] = dupChoice
		if err := c.session.PostJSON(baseURL+"/b/api/file/upload_request", uploadReq, &uploadResp); err != nil {
			return 0, err
		}
	}
	if uploadResp.Code != 0 {
		return 0, fmt.Errorf("上传请求失败: %s (code=%d)", uploadResp.Message, uploadResp.Code)
	}

	// 秒传
	if uploadResp.Data.Reuse {
		c.RefreshFileList()
		return uploadResp.Data.FileID, nil
	}

	bucket := uploadResp.Data.Bucket
	uploadKey := uploadResp.Data.Key
	uploadID := uploadResp.Data.UploadId
	fileID := uploadResp.Data.FileID
	storageNode := uploadResp.Data.StorageNode

	// 2. 初始化分片上传会话
	startData := map[string]interface{}{
		"bucket":      bucket,
		"key":         uploadKey,
		"uploadId":    uploadID,
		"storageNode": storageNode,
	}
	if err := c.session.PostJSON(baseURL+"/b/api/file/s3_list_upload_parts", startData, nil); err != nil {
		return 0, err
	}

	// 3. 分块上传
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var totalUploaded int64
	partNumber := 1

	for {
		select {
		case <-task.Cancel:
			return 0, fmt.Errorf("上传已取消")
		case <-task.Pause:
			<-task.Resume
		default:
		}

		buf := make([]byte, blockSize)
		n, readErr := f.Read(buf)
		if n == 0 {
			break
		}

		// 获取分片上传链接
		getLinkData := map[string]interface{}{
			"bucket":          bucket,
			"key":             uploadKey,
			"partNumberEnd":   partNumber + 1,
			"partNumberStart": partNumber,
			"uploadId":        uploadID,
			"StorageNode":     storageNode,
		}

		var partResp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				PresignedUrls map[string]string `json:"presignedUrls"`
			} `json:"data"`
		}
		if err := c.session.PostJSON(baseURL+"/b/api/file/s3_repare_upload_parts_batch", getLinkData, &partResp); err != nil {
			return 0, err
		}
		if partResp.Code != 0 {
			return 0, fmt.Errorf("获取上传链接失败: %s", partResp.Message)
		}

		partURL := partResp.Data.PresignedUrls[fmt.Sprintf("%d", partNumber)]
		if partURL == "" {
			return 0, fmt.Errorf("获取分片 %d 上传链接失败", partNumber)
		}

		// 上传分片
		putReq, _ := newHTTPRequest("PUT", partURL, buf[:n])
		putReq.Header.Set("Content-Type", "application/octet-stream")
		putResp, err := c.session.Transfer().Do(putReq)
		if err != nil {
			return 0, fmt.Errorf("上传分片 %d 失败: %w", partNumber, err)
		}
		putResp.Body.Close()

		totalUploaded += int64(n)
		partNumber++

		if callback != nil {
			callback(totalUploaded, fsize)
		}
		if task != nil && fsize > 0 {
			task.Progress = int(totalUploaded * 100 / fsize)
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, readErr
		}
	}

	// 4. 列出上传分片
	listPartsData := map[string]interface{}{
		"bucket":      bucket,
		"key":         uploadKey,
		"uploadId":    uploadID,
		"storageNode": storageNode,
	}
	if err := c.session.PostJSON(baseURL+"/b/api/file/s3_list_upload_parts", listPartsData, nil); err != nil {
		return 0, err
	}

	// 5. 完成分片上传
	completeData := map[string]interface{}{
		"bucket":      bucket,
		"key":         uploadKey,
		"uploadId":    uploadID,
		"storageNode": storageNode,
	}
	var compResp model.UploadCompleteResponse
	if err := c.session.PostJSON(baseURL+"/b/api/file/s3_complete_multipart_upload", completeData, &compResp); err != nil {
		return 0, err
	}

	// 大文件等待服务器处理
	if fsize > 64*1024*1024 {
		time.Sleep(3 * time.Second)
	}

	// 6. 确认上传完成
	closeData := map[string]interface{}{"fileId": fileID}
	var closeResp model.UploadCompleteResponse
	if err := c.session.PostJSON(baseURL+"/b/api/file/upload_complete", closeData, &closeResp); err != nil {
		return 0, err
	}
	if closeResp.Code != 0 {
		// 解析详细错误信息
		errDetail, _ := json.Marshal(closeResp)
		return 0, fmt.Errorf("上传完成确认失败: %s", string(errDetail))
	}

	c.RefreshFileList()
	return fileID, nil
}

// newHTTPRequest creates an HTTP request with body bytes
func newHTTPRequest(method, url string, body []byte) (*http.Request, error) {
	return http.NewRequest(method, url, io.NopCloser(io.Reader(&byteReader{data: body})))
}

// byteReader implements io.Reader for a byte slice
type byteReader struct {
	data  []byte
	index int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.index >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.index:])
	r.index += n
	return n, nil
}
