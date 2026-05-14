package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

const baseURL = "https://www.123pan.com"

// ----------------------
// Client
// ----------------------

type Client struct {
    Username      string
    Password      string
    Authorization string
    HTTPClient    *http.Client
}

func NewClient(username, password string) *Client {
    return &Client{
        Username: username,
        Password: password,
        HTTPClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (c *Client) defaultHeaders(req *http.Request) {
    req.Header.Set("User-Agent", "123pan-go-cli")
    req.Header.Set("Content-Type", "application/json")
    if c.Authorization != "" {
        req.Header.Set("Authorization", c.Authorization)
    }
}

// ----------------------
// Login
// ----------------------

type LoginResp struct {
    Code int    `json:"code"`
    Msg  string `json:"message"`
    Data struct {
        Token string `json:"token"`
    } `json:"data"`
}

func (c *Client) Login() error {
    form := fmt.Sprintf("type=1&passport=%s&password=%s", c.Username, c.Password)

    req, err := http.NewRequest("POST", baseURL+"/b/api/user/sign_in", strings.NewReader(form))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("User-Agent", "123pan-go-cli")

    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    var result LoginResp
    if err := json.Unmarshal(body, &result); err != nil {
        return err
    }

    if result.Code != 200 {
        return fmt.Errorf("login failed: %s", result.Msg)
    }

    c.Authorization = "Bearer " + result.Data.Token
    fmt.Println("登录成功")
    return nil
}

func (c *Client) Logout() {
    c.Authorization = ""
    fmt.Println("已退出登录")
}

// ----------------------
// List Files
// ----------------------

type FileItem struct {
    FileID   int64  `json:"FileId"`
    FileName string `json:"FileName"`
    Type     int    `json:"Type"`
    Size     int64  `json:"Size"`
    Etag     string `json:"Etag"`
    S3KeyFlag string `json:"S3KeyFlag"`
}

type ListResp struct {
    Code int `json:"code"`
    Data struct {
        InfoList []FileItem `json:"InfoList"`
    } `json:"data"`
}

func (c *Client) ListFiles() ([]FileItem, error) {
    url := baseURL + "/api/file/list/new?driveId=0&limit=100&next=0&orderBy=file_id&orderDirection=desc&parentFileId=0&trashed=false&Page=1"

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }

    c.defaultHeaders(req)

    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    var result ListResp
    if err := json.Unmarshal(body, &result); err != nil {
        return nil, err
    }

    if result.Code != 0 {
        return nil, fmt.Errorf("list files failed")
    }

    return result.Data.InfoList, nil
}

// ----------------------
// Get Download Link
// ----------------------

type DownloadResp struct {
    Code int `json:"code"`
    Data struct {
        DownloadURL string `json:"DownloadUrl"`
    } `json:"data"`
}

func (c *Client) GetDownloadLink(file FileItem) (string, error) {
    payload := map[string]interface{}{
        "driveId":   0,
        "etag":      file.Etag,
        "fileId":    file.FileID,
        "s3keyFlag": file.S3KeyFlag,
        "type":      file.Type,
        "fileName":  file.FileName,
        "size":      file.Size,
    }

    jsonData, _ := json.Marshal(payload)

    req, err := http.NewRequest("POST", baseURL+"/a/api/file/download_info", bytes.NewBuffer(jsonData))
    if err != nil {
        return "", err
    }

    c.defaultHeaders(req)

    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    var result DownloadResp
    if err := json.Unmarshal(body, &result); err != nil {
        return "", err
    }

    if result.Code != 0 {
        return "", fmt.Errorf("get link failed")
    }

    return result.Data.DownloadURL, nil
}

// ----------------------
// Download File
// ----------------------

func (c *Client) DownloadFile(url, filename string) error {
    resp, err := c.HTTPClient.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if err := os.MkdirAll("downloads", os.ModePerm); err != nil {
        return err
    }

    path := filepath.Join("downloads", filename)
    out, err := os.Create(path)
    if err != nil {
        return err
    }
    defer out.Close()

    _, err = io.Copy(out, resp.Body)
    if err != nil {
        return err
    }

    fmt.Println("下载完成:", path)
    return nil
}

// ----------------------
// CLI
// ----------------------

func main() {
    var username, password string

    fmt.Print("请输入账号: ")
    fmt.Scanln(&username)

    fmt.Print("请输入密码: ")
    fmt.Scanln(&password)

    client := NewClient(username, password)

    if err := client.Login(); err != nil {
        fmt.Println("登录失败:", err)
        return
    }

    files, err := client.ListFiles()
    if err != nil {
        fmt.Println("获取文件失败:", err)
        return
    }

    fmt.Println("\n文件列表：")
    for i, f := range files {
        fmt.Printf("[%d] %s\n", i+1, f.FileName)
    }

    fmt.Print("\n输入文件编号获取直链并下载(输入0退出): ")
    var input string
    fmt.Scanln(&input)

    if input == "0" {
        client.Logout()
        return
    }

    idx, err := strconv.Atoi(input)
    if err != nil || idx < 1 || idx > len(files) {
        fmt.Println("编号无效")
        return
    }

    target := files[idx-1]

    link, err := client.GetDownloadLink(target)
    if err != nil {
        fmt.Println("获取链接失败:", err)
        return
    }

    fmt.Println("\n文件直链：")
    fmt.Println(link)

    if err := client.DownloadFile(link, target.FileName); err != nil {
        fmt.Println("下载失败:", err)
        return
    }

    client.Logout()
}
