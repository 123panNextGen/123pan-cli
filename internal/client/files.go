//Copyright (C) 2026 123panNextGen
//[https://github.com/123panNextGen/123pan-cli]
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"123pan-cli/internal/model"
)

// ListFiles 获取文件列表（支持分页）
func (c *Client) ListFiles(parentFileID int64) ([]model.FileItem, error) {
	return c.listFilesPaged(parentFileID, 1, 100)
}

// listFilesPaged 分页获取文件列表
func (c *Client) listFilesPaged(parentFileID int64, page, limit int) ([]model.FileItem, error) {
	url := fmt.Sprintf(
		"%s/api/file/list/new?driveId=0&limit=%d&next=0&orderBy=file_id&orderDirection=desc&parentFileId=%d&trashed=false&SearchData=&Page=%d&OnlyLookAbnormalFile=0",
		baseURL, limit, parentFileID, page,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.defaultHeadersForReq(req, "application/json")

	resp, err := c.session.HTTP().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result model.FileListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("文件列表解析失败: %w", err)
	}

	if result.Code == 2 {
		return nil, fmt.Errorf("token 过期，请重新登录")
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("获取文件列表失败: %s (code=%d)", result.Message, result.Code)
	}

	c.totalFiles = result.Data.Total
	return result.Data.InfoList, nil
}

// ListAllFiles 获取所有文件（自动分页，带速率限制）
func (c *Client) ListAllFiles(parentFileID int64) ([]model.FileItem, error) {
	var allFiles []model.FileItem
	page := 1

	for {
		files, err := c.listFilesPaged(parentFileID, page, 100)
		if err != nil {
			return allFiles, err
		}
		allFiles = append(allFiles, files...)

		if len(allFiles) >= c.totalFiles || len(files) == 0 {
			break
		}
		page++

		if page%5 == 0 {
			time.Sleep(3 * time.Second) // 防限流
		}
	}
	c.fileList = allFiles
	c.allFiles = true
	return allFiles, nil
}

// RefreshFileList 刷新当前目录的文件列表
func (c *Client) RefreshFileList() error {
	files, err := c.ListFiles(c.currentDirID)
	if err != nil {
		return err
	}
	c.fileList = files
	return nil
}

// ChangeDir 进入指定目录
func (c *Client) ChangeDir(dirID int64, dirName string) error {
	c.currentDirID = dirID
	c.folderStack = append(c.folderStack, dirID)
	c.currentPath = append(c.currentPath, dirName)
	c.filePage = 0
	c.allFiles = false
	return c.RefreshFileList()
}

// ChangeDirByIndex 按索引进入目录 (1-based)
func (c *Client) ChangeDirByIndex(index int) error {
	if index < 1 || index > len(c.fileList) {
		return fmt.Errorf("索引 %d 超出范围 [1, %d]", index, len(c.fileList))
	}
	item := c.fileList[index-1]
	if !item.IsDir() {
		return fmt.Errorf("'%s' 不是文件夹", item.FileName)
	}
	return c.ChangeDir(item.FileID, item.FileName)
}

// GoBack 返回上级目录
func (c *Client) GoBack() error {
	if len(c.folderStack) <= 1 {
		return fmt.Errorf("已在根目录")
	}
	c.folderStack = c.folderStack[:len(c.folderStack)-1]
	c.currentPath = c.currentPath[:len(c.currentPath)-1]
	c.currentDirID = c.folderStack[len(c.folderStack)-1]
	c.filePage = 0
	c.allFiles = false
	return c.RefreshFileList()
}

// GoToRoot 回到根目录
func (c *Client) GoToRoot() error {
	c.currentDirID = 0
	c.folderStack = []int64{0}
	c.currentPath = []string{"根目录"}
	c.filePage = 0
	c.allFiles = false
	return c.RefreshFileList()
}

// CreateDir 创建文件夹
func (c *Client) CreateDir(dirName string) (int64, error) {
	data := map[string]interface{}{
		"driveId":      0,
		"etag":         "",
		"fileName":     dirName,
		"parentFileId": c.currentDirID,
		"size":         0,
		"type":         1,
		"duplicate":    1,
		"NotReuse":     true,
		"event":        "newCreateFolder",
		"operateType":  1,
	}

	var result model.MkdirResponse
	if err := c.session.PostJSON(baseURL+"/a/api/file/upload_request", data, &result); err != nil {
		return 0, err
	}
	if result.Code != 0 {
		return 0, fmt.Errorf("创建文件夹失败: %s (code=%d)", result.Message, result.Code)
	}

	c.RefreshFileList()
	return result.Data.FileID, nil
}

// RenameFile 重命名文件或文件夹
func (c *Client) RenameFile(fileID int64, newName string) error {
	data := map[string]interface{}{
		"driveId":  0,
		"fileId":   fileID,
		"fileName": newName,
	}

	var result model.RenameResponse
	if err := c.session.PostJSON(baseURL+"/a/api/file/rename", data, &result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("重命名失败: %s (code=%d)", result.Message, result.Code)
	}
	c.RefreshFileList()
	return nil
}

// DeleteFile 删除文件（移到回收站）
func (c *Client) DeleteFile(fileID int64) error {
	data := map[string]interface{}{
		"driveId": 0,
		"fileTrashInfoList": []map[string]interface{}{
			{"FileId": fileID},
		},
		"operation": true,
	}

	var result model.TrashResponse
	if err := c.session.PostJSON(baseURL+"/a/api/file/trash", data, &result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("删除失败: %s (code=%d)", result.Message, result.Code)
	}
	c.RefreshFileList()
	return nil
}

// RestoreFile 从回收站恢复文件
func (c *Client) RestoreFile(fileID int64) error {
	data := map[string]interface{}{
		"driveId": 0,
		"fileTrashInfoList": []map[string]interface{}{
			{"FileId": fileID},
		},
		"operation": false,
	}

	var result model.TrashResponse
	if err := c.session.PostJSON(baseURL+"/a/api/file/trash", data, &result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("恢复失败: %s (code=%d)", result.Message, result.Code)
	}
	return nil
}

// GetRecycleBin 获取回收站列表
func (c *Client) GetRecycleBin() ([]model.FileItem, error) {
	url := fmt.Sprintf(
		"%s/api/file/list/new?driveId=0&limit=100&next=0&orderBy=file_id&orderDirection=desc&parentFileId=0&trashed=true&SearchData=&Page=1&OnlyLookAbnormalFile=0",
		baseURL,
	)

	var result model.FileListResponse
	if err := c.session.GetJSON(url, &result); err != nil {
		return nil, err
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("获取回收站失败: %s (code=%d)", result.Message, result.Code)
	}
	c.recycleList = result.Data.InfoList
	return result.Data.InfoList, nil
}

// defaultHeadersForReq 为请求设置默认请求头（内部辅助）
func (c *Client) defaultHeadersForReq(req *http.Request, contentType string) {
	c.session.defaultHeaders(req, contentType)
}
