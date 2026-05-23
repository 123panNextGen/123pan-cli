package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	DeviceType    string
	OSVersion     string
	LoginUUID     string
	HTTPClient    *http.Client
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func NewClient(username, password string) *Client {
	jar, _ := cookiejar.New(nil)
	deviceTypes := []string{"M2102K1C", "2201122C", "2311BPN23C", "2407FPN8EG", "A401XM"}
	osVersions := []string{"Android_13", "Android_12", "Android_11", "Android_10"}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	return &Client{
		Username:   username,
		Password:   password,
		DeviceType: deviceTypes[r.Intn(len(deviceTypes))],
		OSVersion:  osVersions[r.Intn(len(osVersions))],
		LoginUUID:  randomHex(16),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
	}
}

func (c *Client) defaultHeaders(req *http.Request, contentType string) {
	req.Header.Set("User-Agent", fmt.Sprintf("123pan/v2.4.0(%s;Xiaomi)", c.OSVersion))
	//req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("platform", "android")
	req.Header.Set("devicetype", c.DeviceType)
	req.Header.Set("devicename", "Xiaomi")
	req.Header.Set("osversion", c.OSVersion)
	req.Header.Set("app-version", "61")
	req.Header.Set("x-app-version", "2.4.0")
	req.Header.Set("loginuuid", c.LoginUUID)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.Authorization != "" {
		req.Header.Set("authorization", c.Authorization)
	}
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func mustJSONError(prefix string, body []byte) error {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return errors.New(prefix + ": empty response")
	}
	if len(trimmed) > 300 {
		trimmed = trimmed[:300]
	}
	return fmt.Errorf("%s: %s", prefix, trimmed)
}

// ----------------------
// Login
// ----------------------

type LoginResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
	} `json:"data"`
}

func (c *Client) Login() error {
	form := url.Values{}
	form.Set("type", "1")
	form.Set("passport", c.Username)
	form.Set("password", c.Password)

	req, err := http.NewRequest("POST", baseURL+"/b/api/user/sign_in", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	c.defaultHeaders(req, "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	body, err := readBody(resp)
	if err != nil {
		return err
	}

	var result LoginResp
	if err := json.Unmarshal(body, &result); err != nil {
		return mustJSONError("登录响应不是JSON", body)
	}
	if result.Code != 200 {
		if result.Message == "" {
			result.Message = "unknown error"
		}
		return fmt.Errorf("login failed: %s", result.Message)
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
	FileID    int64  `json:"FileId"`
	FileName  string `json:"FileName"`
	Type      int    `json:"Type"`
	Size      int64  `json:"Size"`
	Etag      string `json:"Etag"`
	S3KeyFlag string `json:"S3KeyFlag"`
}

type ListResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		InfoList []FileItem `json:"InfoList"`
		Total    int        `json:"Total"`
	} `json:"data"`
}

func (c *Client) ListFiles(parentFileID int64) ([]FileItem, error) {
	endpoint := fmt.Sprintf(baseURL+"/api/file/list/new?driveId=0&limit=100&next=0&orderBy=file_id&orderDirection=desc&parentFileId=%d&trashed=false&SearchData=&Page=1&OnlyLookAbnormalFile=0", parentFileID)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.defaultHeaders(req, "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var result ListResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, mustJSONError("列表响应不是JSON", body)
	}
	if result.Code != 0 {
		if result.Message == "" {
			result.Message = "list files failed"
		}
		return nil, errors.New(result.Message)
	}
	return result.Data.InfoList, nil
}

func formatPath(path []string) string {
	if len(path) == 0 {
		return "/"
	}
	return "/" + strings.Join(path, "/")
}

// ----------------------
// Download Link
// ----------------------

type DownloadInfoResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		DownloadURL string `json:"DownloadUrl"`
	} `json:"data"`
}

func (c *Client) getRawDownloadURL(file FileItem) (string, error) {
	var endpoint string
	var payload any

	if file.Type == 1 {
		endpoint = baseURL + "/a/api/file/batch_download_info"
		payload = map[string]any{
			"fileIdList": []map[string]int64{{"fileId": file.FileID}},
		}
	} else {
		endpoint = baseURL + "/a/api/file/download_info"
		payload = map[string]any{
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

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}
	c.defaultHeaders(req, "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	body, err := readBody(resp)
	if err != nil {
		return "", err
	}

	var result DownloadInfoResp
	if err := json.Unmarshal(body, &result); err != nil {
		return "", mustJSONError("获取下载信息响应不是JSON", body)
	}
	if result.Code != 0 {
		if result.Message == "" {
			result.Message = "get link failed"
		}
		return "", errors.New(result.Message)
	}
	if result.Data.DownloadURL == "" {
		return "", errors.New("empty DownloadUrl")
	}
	return result.Data.DownloadURL, nil
}

func extractFinalURL(raw string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequest("GET", raw, nil)
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

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`href='(https?://[^']+)'`),
		regexp.MustCompile(`href="(https?://[^"]+)"`),
		regexp.MustCompile(`url=(https?://[^"&']+)`),
	}
	for _, re := range patterns {
		m := re.FindStringSubmatch(txt)
		if len(m) >= 2 {
			return m[1], nil
		}
	}

	if strings.HasPrefix(strings.TrimSpace(txt), "http://") || strings.HasPrefix(strings.TrimSpace(txt), "https://") {
		return strings.TrimSpace(txt), nil
	}

	return "", errors.New("cannot resolve final download url")
}

func (c *Client) GetDownloadLink(file FileItem) (string, error) {
	raw, err := c.getRawDownloadURL(file)
	if err != nil {
		return "", err
	}
	return extractFinalURL(raw)
}

// ----------------------
// Download File
// ----------------------

func (c *Client) DownloadFile(downloadURL, filename, dir string) error {
	if dir == "" {
		dir = "downloads"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	outPath := filepath.Join(dir, filename)
	tmpPath := outPath + ".part"

	if _, err := os.Stat(outPath); err == nil {
		return fmt.Errorf("file already exists: %s", outPath)
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed: %s", strings.TrimSpace(string(b)))
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}

	if err := os.Rename(tmpPath, outPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	fmt.Println("下载完成:", outPath)
	return nil
}

// ----------------------
// Helpers
// ----------------------

func printFiles(files []FileItem) {
	if len(files) == 0 {
		fmt.Println("没有文件")
		return
	}
	for i, f := range files {
		typeName := "文件"
		if f.Type == 1 {
			typeName = "文件夹"
		}
		fmt.Printf("[%d] %s  (%s, %d bytes)\n", i+1, f.FileName, typeName, f.Size)
	}
}

func readLine(scanner *bufio.Scanner, prompt string) string {
	fmt.Print(prompt)
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}

func usage() {
	fmt.Println(`命令：
  list                      列出当前目录文件
  link <编号>               输出文件直链
  download <编号> [目录]    下载文件到目录，默认 downloads
  cd <编号>                 进入文件夹
  cd ..                     返回上一级
  pwd                       显示当前目录
  logout                    退出登录
  help                      显示帮助
  exit / quit               退出程序`)
}

// ----------------------
// CLI
// ----------------------

func main() {
	fmt.Println("123pan-cli v1.0.1")
	fmt.Println("https://github.com/123panNextGen/123pan-cli")
	fmt.Println()
	usage()
	fmt.Println()

	var username, password string
	flag.StringVar(&username, "u", "", "账号")
	flag.StringVar(&password, "p", "", "密码")
	flag.Parse()

	scanner := bufio.NewScanner(os.Stdin)

	if username == "" {
		username = readLine(scanner, "请输入账号: ")
	}
	if password == "" {
		password = readLine(scanner, "请输入密码: ")
	}
	if username == "" || password == "" {
		fmt.Println("账号或密码为空")
		return
	}

	client := NewClient(username, password)
	if err := client.Login(); err != nil {
		fmt.Println("登录失败:", err)
		return
	}

	currentPath := []string{"根目录"}
	currentFolderIDs := []int64{0}
	currentParentID := int64(0)

	files, err := client.ListFiles(currentParentID)
	if err != nil {
		fmt.Println("获取文件失败:", err)
		return
	}

	for {
		line := readLine(scanner, "123pan> ")
		if line == "" {
			continue
		}
		args := strings.Fields(line)
		switch strings.ToLower(args[0]) {
		case "list", "ls":
			files, err = client.ListFiles(currentParentID)
			if err != nil {
				fmt.Println("获取文件失败:", err)
				continue
			}
			printFiles(files)

		case "pwd":
			fmt.Println(formatPath(currentPath))

		case "cd":
			if len(args) < 2 {
				fmt.Println("用法: cd <编号> 或 cd ..")
				continue
			}
			if args[1] == ".." {
				if len(currentPath) <= 1 {
					fmt.Println("已在根目录")
					continue
				}
				currentPath = currentPath[:len(currentPath)-1]
				currentFolderIDs = currentFolderIDs[:len(currentFolderIDs)-1]
				currentParentID = currentFolderIDs[len(currentFolderIDs)-1]
				files, err = client.ListFiles(currentParentID)
				if err != nil {
					fmt.Println("获取文件失败:", err)
					continue
				}
				continue
			}

			idx, err := strconv.Atoi(args[1])
			if err != nil || idx < 1 || idx > len(files) {
				fmt.Println("编号无效")
				continue
			}
			item := files[idx-1]
			if item.Type != 1 {
				fmt.Println("不是文件夹")
				continue
			}
			currentParentID = item.FileID
			currentFolderIDs = append(currentFolderIDs, currentParentID)
			currentPath = append(currentPath, item.FileName)
			files, err = client.ListFiles(currentParentID)
			if err != nil {
				fmt.Println("获取文件失败:", err)
				continue
			}

		case "link":
			if len(args) < 2 {
				fmt.Println("用法: link <编号>")
				continue
			}
			idx, err := strconv.Atoi(args[1])
			if err != nil || idx < 1 || idx > len(files) {
				fmt.Println("编号无效")
				continue
			}
			link, err := client.GetDownloadLink(files[idx-1])
			if err != nil {
				fmt.Println("获取链接失败:", err)
				continue
			}
			fmt.Println(link)

		case "download":
			if len(args) < 2 {
				fmt.Println("用法: download <编号> [目录]")
				continue
			}
			idx, err := strconv.Atoi(args[1])
			if err != nil || idx < 1 || idx > len(files) {
				fmt.Println("编号无效")
				continue
			}
			dir := "downloads"
			if len(args) >= 3 {
				dir = args[2]
			}
			link, err := client.GetDownloadLink(files[idx-1])
			if err != nil {
				fmt.Println("获取链接失败:", err)
				continue
			}
			if err := client.DownloadFile(link, files[idx-1].FileName, dir); err != nil {
				fmt.Println("下载失败:", err)
				continue
			}

		case "logout":
			client.Logout()

		case "help":
			usage()

		case "exit", "quit":
			if client.Authorization != "" {
				client.Logout()
			}
			return

		default:
			fmt.Println("未知命令，输入 help 查看帮助")
		}
	}
}
