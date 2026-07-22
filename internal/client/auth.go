//Copyright (C) 2026 123panNextGen
//[https://github.com/123panNextGen/123pan-cli]
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"123pan-cli/internal/model"
)

// Login 登录
func (c *Client) Login() error {
	loginBody := map[string]interface{}{
		"type":     1,
		"passport": c.session.UserInfo().UserName,
		"password": c.session.UserInfo().Password,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(loginBody); err != nil {
		return fmt.Errorf("序列化登录请求失败: %w", err)
	}
	bodyBytes := bytes.TrimRight(buf.Bytes(), "\n")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/b/api/user/sign_in", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	c.defaultHeadersForReq(req, "application/json")

	resp, err := c.session.HTTP().Do(req)
	if err != nil {
		return fmt.Errorf("登录请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result model.LoginResponse
	if err := json.Unmarshal(body, &result); err != nil {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return fmt.Errorf("登录响应解析失败 (HTTP %d): %w\n响应: %s", resp.StatusCode, err, preview)
	}

	if result.Code != 200 {
		return fmt.Errorf("登录失败: %s (code=%d)", result.Message, result.Code)
	}

	c.session.UserInfo().Authorization = "Bearer " + result.Data.Token

	// 保存账号到配置
	acc := model.Account{
		UserName:      c.session.UserInfo().UserName,
		Password:      c.session.UserInfo().Password,
		Authorization: c.session.UserInfo().Authorization,
		DeviceType:    c.session.UserInfo().Device.Type,
		OSVersion:     c.session.UserInfo().Device.OS,
		LoginUUID:     c.session.UserInfo().UUID,
	}
	model.SaveAccount(acc)
	return nil
}

// LoginWithCredentials 用指定凭据登录
func (c *Client) LoginWithCredentials(userName, password string) error {
	c.session.UserInfo().UserName = userName
	c.session.UserInfo().Password = password
	return c.Login()
}

// Logout 登出
func (c *Client) Logout() {
	c.session.UserInfo().Authorization = ""
}

// IsLoggedIn 检查是否已登录
func (c *Client) IsLoggedIn() bool {
	return c.session.Authorization() != ""
}

// Authorization 返回授权 token
func (c *Client) Authorization() string {
	return c.session.Authorization()
}
