package client

import (
	"fmt"
	"math/rand"
	"time"

	"123pan-cli/internal/model"
	"123pan-cli/internal/utils"
)

const baseURL = "https://www.123pan.cn"

var deviceTypes = []string{"M2102K1C", "2201122C", "2311BPN23C", "2407FPN8EG", "A401XM"}
var osVersions = []string{"Android_13", "Android_12", "Android_11", "Android_10"}

// Client 123云盘客户端（门面）
type Client struct {
	session      *Session
	currentPath  []string
	folderStack  []int64 // 目录 ID 栈
	currentDirID int64

	fileList    []model.FileItem
	filePage    int
	allFiles    bool
	totalFiles  int
	recycleList []model.FileItem
}

// NewClient 创建新的客户端实例
func NewClient() *Client {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	s := NewSession()
	info := &model.UserInfo{
		UserName: "",
		Password: "",
		UUID:     utils.RandomHex(16),
		Device: model.DeviceInfo{
			OS:   osVersions[r.Intn(len(osVersions))],
			Type: deviceTypes[r.Intn(len(deviceTypes))],
		},
	}
	s.SetUserInfo(info)

	return &Client{
		session:      s,
		currentPath:  []string{"根目录"},
		folderStack:  []int64{0},
		currentDirID: 0,
		filePage:     0,
	}
}

// NewClientFromConfig 从配置创建客户端
func NewClientFromConfig(userName string) (*Client, error) {
	c := NewClient()
	if userName == "" {
		acc := model.GetAccount("")
		if acc == nil {
			return nil, fmt.Errorf("没有找到已保存的账号")
		}
		userName = acc.UserName
	}

	acc := model.GetAccount(userName)
	if acc == nil {
		return nil, fmt.Errorf("没有找到账号: %s", userName)
	}

	c.session.UserInfo().UserName = acc.UserName
	c.session.UserInfo().Password = acc.Password
	c.session.UserInfo().Authorization = acc.Authorization
	c.session.UserInfo().Device.OS = acc.OSVersion
	c.session.UserInfo().Device.Type = acc.DeviceType
	c.session.UserInfo().UUID = acc.LoginUUID

	if acc.Authorization != "" {
		// 尝试获取文件列表验证 token 是否有效
		_, err := c.ListFiles(0)
		if err != nil {
			// token 过期，重新登录
			if err := c.Login(); err != nil {
				return nil, err
			}
		}
	}
	return c, nil
}

// Session 返回内部 Session
func (c *Client) Session() *Session {
	return c.session
}

// CurrentDirID 返回当前目录 ID
func (c *Client) CurrentDirID() int64 {
	return c.currentDirID
}

// CurrentPath 返回当前路径
func (c *Client) CurrentPath() string {
	if len(c.currentPath) == 0 {
		return "/"
	}
	path := ""
	for _, p := range c.currentPath {
		path += "/" + p
	}
	return path
}

// FileList 返回当前文件列表
func (c *Client) FileList() []model.FileItem {
	return c.fileList
}

// TotalFiles 返回文件总数
func (c *Client) TotalFiles() int {
	return c.totalFiles
}

// GetRecycleBinCached 返回缓存的回收站列表
func (c *Client) GetRecycleBinCached() []model.FileItem {
	return c.recycleList
}
