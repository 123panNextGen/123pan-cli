//Copyright (C) 2026 123panNextGen
//[https://github.com/123panNextGen/123pan-cli]
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package model

import (
	"encoding/json"
	"strconv"
	"time"
)

// ---------- API 返回 ----------

type ApiCode int

const (
	ApiCodeSuccess ApiCode = iota
	ApiCodeFail
)

type ApiReturn[T any] struct {
	Code        int     `json:"code"`
	ApiCode     int     `json:"apiCode"`
	ApiCodeEnum ApiCode `json:"-"`
	Msg         string  `json:"message"`
	Data        T       `json:"data"`
}

func (a *ApiReturn[T]) IsSuccess() bool {
	return a.Code == 0 || a.Code == 200
}

// ---------- 登录 ----------

type LoginData struct {
	Token         string             `json:"token"`
	Authorization string             `json:"authorization"`
	Cookies       map[string]*string `json:"cookies"`
}

type LoginResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
	} `json:"data"`
}

// ---------- 文件项 ----------

type FileItem struct {
	FileID        int64  `json:"FileId"`
	FileName      string `json:"FileName"`
	Type          int    `json:"Type"` // 0=文件, 1=文件夹
	Size          int64  `json:"Size"`
	CreateAt      int64  `json:"CreateAt"`
	UpdateAt      int64  `json:"UpdateAt"`
	Hidden        bool   `json:"Hidden"`
	Etag          string `json:"Etag"`
	S3KeyFlag     string `json:"S3KeyFlag"`
	ContentType   string `json:"ContentType"`
	ParentFileID  int64  `json:"ParentFileId"`
	PinYin        string `json:"PinYin"`
	StarredStatus bool   `json:"StarredStatus"`
}

func (f FileItem) IsDir() bool           { return f.Type == 1 }
func (f FileItem) CreateTime() time.Time { return time.Unix(f.CreateAt, 0) }
func (f FileItem) UpdateTime() time.Time { return time.Unix(f.UpdateAt, 0) }

func (f FileItem) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"FileId":        f.FileID,
		"FileName":      f.FileName,
		"Type":          f.Type,
		"Size":          f.Size,
		"CreateAt":      f.CreateAt,
		"UpdateAt":      f.UpdateAt,
		"Hidden":        f.Hidden,
		"Etag":          f.Etag,
		"S3KeyFlag":     f.S3KeyFlag,
		"ContentType":   f.ContentType,
		"ParentFileId":  f.ParentFileID,
		"PinYin":        f.PinYin,
		"StarredStatus": f.StarredStatus,
	}
}

// FileItemJSON is used for unmarshaling with flexible field types.
type fileItemJSON struct {
	FileID        json.RawMessage `json:"FileId"`
	FileName      string          `json:"FileName"`
	Type          json.RawMessage `json:"Type"`
	Size          json.RawMessage `json:"Size"`
	CreateAt      json.RawMessage `json:"CreateAt"`
	UpdateAt      json.RawMessage `json:"UpdateAt"`
	Hidden        json.RawMessage `json:"Hidden"`
	Etag          string          `json:"Etag"`
	S3KeyFlag     string          `json:"S3KeyFlag"`
	ContentType   string          `json:"ContentType"`
	ParentFileID  json.RawMessage `json:"ParentFileId"`
	PinYin        string          `json:"PinYin"`
	StarredStatus json.RawMessage `json:"StarredStatus"`
}

func parseInt64OrZero(raw json.RawMessage) int64 {
	var i int64
	if err := json.Unmarshal(raw, &i); err == nil {
		return i
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		v, _ := strconv.ParseInt(s, 10, 64)
		return v
	}
	return 0
}

func parseIntOrZero(raw json.RawMessage) int {
	return int(parseInt64OrZero(raw))
}

func parseBoolOrFalse(raw json.RawMessage) bool {
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		v, _ := strconv.ParseBool(s)
		return v
	}
	return false
}

func (f *FileItem) UnmarshalJSON(data []byte) error {
	var raw fileItemJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.FileID = parseInt64OrZero(raw.FileID)
	f.FileName = raw.FileName
	f.Type = parseIntOrZero(raw.Type)
	f.Size = parseInt64OrZero(raw.Size)
	f.CreateAt = parseInt64OrZero(raw.CreateAt)
	f.UpdateAt = parseInt64OrZero(raw.UpdateAt)
	f.Hidden = parseBoolOrFalse(raw.Hidden)
	f.Etag = raw.Etag
	f.S3KeyFlag = raw.S3KeyFlag
	f.ContentType = raw.ContentType
	f.ParentFileID = parseInt64OrZero(raw.ParentFileID)
	f.PinYin = raw.PinYin
	f.StarredStatus = parseBoolOrFalse(raw.StarredStatus)
	return nil
}

// ---------- 文件列表 ----------

type FileListData struct {
	Next     string     `json:"Next"`
	Len      int        `json:"Len"`
	Total    int        `json:"Total"`
	IsFirst  bool       `json:"IsFirst"`
	InfoList []FileItem `json:"InfoList"`
}

type FileListResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    FileListData `json:"data"`
}

// ---------- 下载 ----------

type DownloadInfoData struct {
	DownloadURL string `json:"DownloadUrl"`
	RedirectUrl string `json:"RedirectUrl"`
	RedirectURL string `json:"redirect_url"` // CDN 可能返回小写字段
}

// ResolvedDownloadURL 返回最终下载 URL（优先 RedirectUrl → redirect_url → DownloadUrl）
func (d DownloadInfoData) ResolvedDownloadURL() string {
	if d.RedirectUrl != "" {
		return d.RedirectUrl
	}
	if d.RedirectURL != "" {
		return d.RedirectURL
	}
	return d.DownloadURL
}

type DownloadInfoResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    DownloadInfoData `json:"data"`
}

// ---------- 上传 ----------

type UploadRequestData struct {
	FileID      int64  `json:"FileId"`
	Bucket      string `json:"Bucket"`
	Key         string `json:"Key"`
	UploadId    string `json:"UploadId"`
	StorageNode string `json:"StorageNode"`
	Reuse       bool   `json:"Reuse"`
}

type UploadRequestResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Data    UploadRequestData `json:"data"`
}

type UploadPartData struct {
	PresignedUrls map[string]string `json:"presignedUrls"`
}

type UploadPartResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    UploadPartData `json:"data"`
}

// ---------- 分享 ----------

type ShareCreateData struct {
	ShareKey string `json:"ShareKey"`
}

type ShareCreateResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    ShareCreateData `json:"data"`
}

// ---------- 创建文件夹 ----------

type MkdirData struct {
	FileID int64     `json:"FileId"`
	Info   *FileItem `json:"Info"`
}

type MkdirResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    MkdirData `json:"data"`
}

// ---------- 重命名 ----------

type RenameResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------- 回收站 ----------

type TrashResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------- 上传完成 ----------

type UploadCompleteResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------- GitHub 版本检查 ----------

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

// ---------- 设备/用户 ----------

type DeviceInfo struct {
	OS   string `json:"os"`
	Type string `json:"type"`
}

type UserInfo struct {
	UserName      string     `json:"userName"`
	Password      string     `json:"password"`
	Authorization string     `json:"authorization"`
	UUID          string     `json:"uuid"`
	Device        DeviceInfo `json:"device"`
}

// ---------- 传输任务 ----------

type TaskType string

const (
	TaskDownload TaskType = "下载"
	TaskUpload   TaskType = "上传"
)

type TaskStatus string

const (
	TaskWaiting   TaskStatus = "等待中"
	TaskRunning   TaskStatus = "进行中"
	TaskPaused    TaskStatus = "已暂停"
	TaskCompleted TaskStatus = "已完成"
	TaskCancelled TaskStatus = "已取消"
	TaskFailed    TaskStatus = "失败"
)

type TransferTask struct {
	ID       int           `json:"id"`
	Type     TaskType      `json:"type"`
	Name     string        `json:"name"`
	Size     int64         `json:"size"`
	Progress int           `json:"progress"` // 0-100
	Status   TaskStatus    `json:"status"`
	FilePath string        `json:"filePath"`
	Cancel   chan struct{} `json:"-"`
	Pause    chan struct{} `json:"-"`
	Resume   chan struct{} `json:"-"`
}

func NewTransferTask(id int, taskType TaskType, name string, size int64) *TransferTask {
	return &TransferTask{
		ID:       id,
		Type:     taskType,
		Name:     name,
		Size:     size,
		Progress: 0,
		Status:   TaskWaiting,
		Cancel:   make(chan struct{}, 1),
		Pause:    make(chan struct{}, 1),
		Resume:   make(chan struct{}, 1),
	}
}
