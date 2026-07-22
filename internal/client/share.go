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
	"time"

	"123pan-cli/internal/model"
)

// ShareFile 分享文件
func (c *Client) ShareFile(fileIDList []int64, sharePwd string) (string, error) {
	if len(fileIDList) == 0 {
		return "", fmt.Errorf("文件ID列表为空")
	}

	data := map[string]interface{}{
		"driveId":    0,
		"expiration": "2099-12-12T08:00:00+08:00",
		"fileIdList": fileIDList,
		"shareName":  "123云盘分享",
		"sharePwd":   sharePwd,
		"event":      "shareCreate",
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			ShareKey string `json:"ShareKey"`
		} `json:"data"`
	}

	if err := c.session.PostJSON(baseURL+"/a/api/share/create", data, &result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("分享失败: %s", result.Message)
	}

	shareURL := "https://www.123pan.cn/s/" + result.Data.ShareKey
	return shareURL, nil
}

// ShareFileByIndex 按索引分享当前目录文件
func (c *Client) ShareFileByIndex(index int, sharePwd string) (string, error) {
	if index < 1 || index > len(c.fileList) {
		return "", fmt.Errorf("索引 %d 超出范围", index)
	}
	return c.ShareFile([]int64{c.fileList[index-1].FileID}, sharePwd)
}

// GetDownloadInfo 获取文件下载信息
func (c *Client) GetDownloadInfo(file model.FileItem) (*model.DownloadInfoResponse, error) {
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
		return nil, err
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("获取下载信息失败: %s", result.Message)
	}
	return &result, nil
}

// CheckVersion 检查最新版本
func CheckVersion(currentVersion string) (bool, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/123panNextGen/123pan/releases/latest", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "123pan-cli")

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var release model.GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return false, err
	}

	return release.Name == currentVersion, nil
}
