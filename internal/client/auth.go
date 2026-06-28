package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"123pan-cli/internal/model"
)

// Login 登录
func (c *Client) Login() error {
	form := url.Values{}
	form.Set("type", "1")
	form.Set("passport", c.session.UserInfo().UserName)
	form.Set("password", c.session.UserInfo().Password)

	resp, err := c.session.Do("POST", baseURL+"/b/api/user/sign_in", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result model.LoginResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("登录响应解析失败: %w", err)
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
