package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"123pan-cli/internal/model"
)

// Session HTTP 会话管理层，封装所有 HTTP 请求
type Session struct {
	http     *http.Client
	transfer *http.Client // 用于大文件传输，不携带鉴权头
	userInfo *model.UserInfo
}

// NewSession 创建新的 HTTP 会话
func NewSession() *Session {
	jar, _ := cookiejar.New(nil)

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
	}
	transferClient := &http.Client{
		Timeout: 300 * time.Second, // 大文件传输需要更长超时
		Jar:     jar,
	}

	s := &Session{
		http:     httpClient,
		transfer: transferClient,
	}
	return s
}

// SetUserInfo 设置用户信息
func (s *Session) SetUserInfo(info *model.UserInfo) {
	s.userInfo = info
}

// UserInfo 获取当前用户信息
func (s *Session) UserInfo() *model.UserInfo {
	return s.userInfo
}

// HTTP 返回 http.Client（用于自定义请求）
func (s *Session) HTTP() *http.Client {
	return s.http
}

// Transfer 返回传输专用 Client
func (s *Session) Transfer() *http.Client {
	return s.transfer
}

// Authorization 返回当前鉴权 token
func (s *Session) Authorization() string {
	if s.userInfo != nil {
		return s.userInfo.Authorization
	}
	return ""
}

// defaultHeaders 设置默认请求头
func (s *Session) defaultHeaders(req *http.Request, contentType string) {
	req.Header.Set("platform", "android")
	req.Header.Set("devicename", "Xiaomi")
	req.Header.Set("app-version", "61")
	req.Header.Set("x-app-version", "2.4.0")

	if s.userInfo != nil {
		req.Header.Set("User-Agent", fmt.Sprintf("123pan/v2.4.0(%s;Xiaomi)", s.userInfo.Device.OS))
		req.Header.Set("osversion", s.userInfo.Device.OS)
		req.Header.Set("devicetype", s.userInfo.Device.Type)
		req.Header.Set("loginuuid", s.userInfo.UUID)
		if s.userInfo.Authorization != "" {
			req.Header.Set("authorization", s.userInfo.Authorization)
		}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
}

// Do 发送带默认头的请求
func (s *Session) Do(method, url string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		switch v := body.(type) {
		case []byte:
			bodyReader = bytes.NewReader(v)
		case string:
			bodyReader = strings.NewReader(v)
		case io.Reader:
			bodyReader = v
		default:
			data, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewReader(data)
		}
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	s.defaultHeaders(req, "application/json")
	return s.http.Do(req)
}

// DoTransfer 发送传输请求（不携带鉴权头）
func (s *Session) DoTransfer(method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return s.transfer.Do(req)
}

// PostJSON 发送 POST JSON 请求并解析响应
func (s *Session) PostJSON(url string, body interface{}, result interface{}) error {
	resp, err := s.Do("POST", url, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}

// GetJSON 发送 GET 请求并解析响应
func (s *Session) GetJSON(url string, result interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	s.defaultHeaders(req, "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}

// SetProxy 设置 HTTP 代理
func (s *Session) SetProxy(proxyURL string) {
	// 重新创建 transport 以应用代理
	if proxyURL != "" {
		s.http.Transport = &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(proxyURL)
			},
		}
		s.transfer.Transport = &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(proxyURL)
			},
		}
	} else {
		s.http.Transport = nil
		s.transfer.Transport = nil
	}
}
