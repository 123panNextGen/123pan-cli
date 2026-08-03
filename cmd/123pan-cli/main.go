//Copyright (C) 2026 123panNextGen
//[https://github.com/123panNextGen/123pan-cli]
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"123pan-cli/internal/client"
	"123pan-cli/internal/model"
	"123pan-cli/internal/tui"
	"123pan-cli/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var (
		username    string
		password    string
		useTUI      bool
		versionFlag bool
	)

	flag.StringVar(&username, "u", "", "账号")
	flag.StringVar(&password, "p", "", "密码")
	flag.BoolVar(&useTUI, "tui", false, "使用终端交互界面 (TUI)")
	flag.BoolVar(&versionFlag, "v", false, "显示版本号")
	flag.BoolVar(&versionFlag, "version", false, "显示版本号")
	flag.Parse()

	if versionFlag {
		fmt.Println("123pan-cli v1.1.3")
		return
	}

	if useTUI {
		runTUI()
		return
	}

	runCLI(username, password)
}

func runTUI() {
	var c *client.Client

	// 尝试从配置加载
	accounts := model.ListAccounts()
	if len(accounts) == 1 && accounts[0].Authorization != "" {
		// 只有一个已授权账号，自动加载
		var err error
		c, err = client.NewClientFromConfig(accounts[0].UserName)
		if err == nil {
			c.RefreshFileList()
		} else {
			c = client.NewClient()
		}
	} else if len(accounts) > 1 {
		// 多个账号，让用户选择
		c = selectAccount(accounts)
		if c == nil {
			c = client.NewClient()
		}
	} else {
		c = client.NewClient()
	}

	m := tui.NewModel(c)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI 错误: %v\n", err)
		os.Exit(1)
	}
}

func runCLI(username, password string) {
	fmt.Println("123pan-cli v1.1.3")
	fmt.Println("https://github.com/123panNextGen/123pan-cli")
	fmt.Println()

	var c *client.Client

	// 尝试从配置加载
	if username == "" && password == "" {
		accounts := model.ListAccounts()
		if len(accounts) > 0 {
			c = selectAccount(accounts)
		}
	}

	if c == nil {
		scanner := bufio.NewScanner(os.Stdin)
		if username == "" {
			fmt.Print("请输入账号: ")
			scanner.Scan()
			username = strings.TrimSpace(scanner.Text())
		}
		if password == "" {
			fmt.Print("请输入密码: ")
			scanner.Scan()
			password = strings.TrimSpace(scanner.Text())
		}
		if username == "" || password == "" {
			fmt.Println("账号或密码为空")
			return
		}

		c = client.NewClient()
		if err := c.LoginWithCredentials(username, password); err != nil {
			fmt.Println("登录失败:", err)
			return
		}
		fmt.Println("登录成功")
	} else if !c.IsLoggedIn() && username != "" && password != "" {
		if err := c.LoginWithCredentials(username, password); err != nil {
			fmt.Println("登录失败:", err)
			return
		}
	}

	if err := c.RefreshFileList(); err != nil {
		fmt.Println("获取文件列表失败:", err)
		return
	}

	printUsage()
	scanner := bufio.NewScanner(os.Stdin)

	for {
		prompt := c.CurrentPath() + "> "
		fmt.Print(prompt)
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		args := strings.Fields(line)
		if !handleCommand(c, args) {
			break
		}
	}
}

func handleCommand(c *client.Client, args []string) bool {
	if len(args) == 0 {
		return true
	}

	switch strings.ToLower(args[0]) {
	case "list", "ls":
		if err := c.RefreshFileList(); err != nil {
			fmt.Println("获取文件列表失败:", err)
			return true
		}
		printFiles(c.FileList())

	case "pwd":
		fmt.Println(c.CurrentPath())

	case "cd":
		if len(args) < 2 {
			fmt.Println("用法: cd <编号> 或 cd ..")
			return true
		}
		if args[1] == ".." {
			if err := c.GoBack(); err != nil {
				fmt.Println(err)
			}
			return true
		}
		if args[1] == "/" {
			if err := c.GoToRoot(); err != nil {
				fmt.Println(err)
			}
			return true
		}
		idx, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("编号无效")
			return true
		}
		if err := c.ChangeDirByIndex(idx); err != nil {
			fmt.Println(err)
		}

	case "link":
		if len(args) < 2 {
			fmt.Println("用法: link <编号>")
			return true
		}
		idx, err := strconv.Atoi(args[1])
		if err != nil || idx < 1 || idx > len(c.FileList()) {
			fmt.Println("编号无效")
			return true
		}
		link, err := c.GetDownloadLink(c.FileList()[idx-1])
		if err != nil {
			fmt.Println("获取链接失败:", err)
			return true
		}
		fmt.Println(link)

	case "download", "dl":
		if len(args) < 2 {
			fmt.Println("用法: download <编号> [目录]")
			return true
		}
		idx, err := strconv.Atoi(args[1])
		if err != nil || idx < 1 || idx > len(c.FileList()) {
			fmt.Println("编号无效")
			return true
		}
		dir := "downloads"
		if len(args) >= 3 {
			dir = args[2]
		}
		file := c.FileList()[idx-1]
		link, err := c.GetDownloadLink(file)
		if err != nil {
			fmt.Println("获取链接失败:", err)
			return true
		}
		fmt.Printf("正在下载: %s (%s)\n", file.FileName, utils.FormatFileSize(file.Size))
		path, err := c.DownloadFile(link, file.FileName, dir)
		if err != nil {
			fmt.Println("下载失败:", err)
			return true
		}
		fmt.Println("下载完成:", path)

	case "upload", "up":
		if len(args) < 2 {
			fmt.Println("用法: upload <文件路径>")
			return true
		}
		filePath := args[1]
		fmt.Printf("正在上传: %s\n", filePath)
		id, err := c.UploadFile(filePath)
		if err != nil {
			fmt.Println("上传失败:", err)
			return true
		}
		fmt.Printf("上传成功 (FileID: %d)\n", id)

	case "mkdir":
		if len(args) < 2 {
			fmt.Println("用法: mkdir <文件夹名>")
			return true
		}
		id, err := c.CreateDir(args[1])
		if err != nil {
			fmt.Println("创建失败:", err)
			return true
		}
		fmt.Printf("文件夹已创建 (ID: %d)\n", id)

	case "rename", "rn":
		if len(args) < 3 {
			fmt.Println("用法: rename <编号> <新名称>")
			return true
		}
		idx, err := strconv.Atoi(args[1])
		if err != nil || idx < 1 || idx > len(c.FileList()) {
			fmt.Println("编号无效")
			return true
		}
		if err := c.RenameFile(c.FileList()[idx-1].FileID, args[2]); err != nil {
			fmt.Println("重命名失败:", err)
			return true
		}
		fmt.Println("重命名成功")

	case "delete", "rm":
		if len(args) < 2 {
			fmt.Println("用法: delete <编号>")
			return true
		}
		idx, err := strconv.Atoi(args[1])
		if err != nil || idx < 1 || idx > len(c.FileList()) {
			fmt.Println("编号无效")
			return true
		}
		fmt.Printf("确定删除 '%s'? (y/N): ", c.FileList()[idx-1].FileName)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) == "y" {
			if err := c.DeleteFile(c.FileList()[idx-1].FileID); err != nil {
				fmt.Println("删除失败:", err)
				return true
			}
			fmt.Println("已删除")
		} else {
			fmt.Println("已取消")
		}

	case "share":
		if len(args) < 2 {
			fmt.Println("用法: share <编号> [密码]")
			return true
		}
		idx, err := strconv.Atoi(args[1])
		if err != nil || idx < 1 || idx > len(c.FileList()) {
			fmt.Println("编号无效")
			return true
		}
		pwd := ""
		if len(args) >= 3 {
			pwd = args[2]
		}
		url, err := c.ShareFileByIndex(idx, pwd)
		if err != nil {
			fmt.Println("分享失败:", err)
			return true
		}
		fmt.Println("分享链接:", url)

	case "recycle", "trash":
		files, err := c.GetRecycleBin()
		if err != nil {
			fmt.Println("获取回收站失败:", err)
			return true
		}
		if len(files) == 0 {
			fmt.Println("回收站为空")
		} else {
			printFiles(files)
		}

	case "restore":
		if len(args) < 2 {
			fmt.Println("用法: restore <编号> (需要先在回收站中)")
			return true
		}
		if c.GetRecycleBinCached() == nil {
			fmt.Println("请先执行 recycle 命令查看回收站")
			return true
		}
		idx, err := strconv.Atoi(args[1])
		if err != nil || idx < 1 || idx > len(c.GetRecycleBinCached()) {
			fmt.Println("编号无效")
			return true
		}
		restoreIdx := idx
		if err := c.RestoreFile(c.GetRecycleBinCached()[restoreIdx-1].FileID); err != nil {
			fmt.Println("恢复失败:", err)
			return true
		}
		fmt.Println("已恢复")

	case "logout":
		c.Logout()
		fmt.Println("已退出登录")

	case "help", "?":
		printUsage()

	case "exit", "quit":
		return false

	case "version":
		fmt.Println("123pan-cli v1.1.3")

	default:
		fmt.Printf("未知命令: %s (输入 help 查看帮助)\n", args[0])
	}
	return true
}

func printUsage() {
	fmt.Println(`命令:
  list / ls                 列出当前目录文件
  link <编号>               获取文件下载直链
  download/dl <编号> [目录] 下载文件到指定目录
  upload/up <路径>          上传文件
  cd <编号>                 进入文件夹
  cd ..                     返回上一级
  cd /                      回到根目录
  pwd                       显示当前路径
  mkdir <名称>              创建文件夹
  rename/rn <编号> <新名>   重命名文件
  delete/rm <编号>          删除文件
  share <编号> [密码]       分享文件
  recycle/trash             查看回收站
  restore <编号>            从回收站恢复
  logout                    退出登录
  version                   显示版本
  help                      显示帮助
  exit/quit                 退出程序`)
}

func printFiles(files []model.FileItem) {
	if len(files) == 0 {
		fmt.Println("(空)")
		return
	}
	for i, f := range files {
		typeName := "文件"
		icon := "📄"
		if f.IsDir() {
			typeName = "文件夹"
			icon = "📁"
		}
		sizeStr := ""
		if !f.IsDir() {
			sizeStr = fmt.Sprintf("  %s", utils.FormatFileSize(f.Size))
		}
		fmt.Printf("[%d] %s %-40s %s%s\n", i+1, icon, f.FileName, typeName, sizeStr)
	}
}

// selectAccount 让用户从已保存的账号中选择
func selectAccount(accounts []model.Account) *client.Client {
	fmt.Println("检测到已保存的账号:")
	for i, acc := range accounts {
		fmt.Printf("  [%d] %s\n", i+1, acc.UserName)
	}
	fmt.Printf("  [%d] 使用新账号登录\n", len(accounts)+1)
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("请选择 (1-%d): ", len(accounts)+1)
		if !scanner.Scan() {
			return nil
		}
		choice := strings.TrimSpace(scanner.Text())

		idx, err := strconv.Atoi(choice)
		if err != nil || idx < 1 || idx > len(accounts)+1 {
			fmt.Println("无效的选择，请重新输入")
			continue
		}

		if idx == len(accounts)+1 {
			// 用户选择使用新账号
			return nil
		}

		// 尝试用已有账号登录
		acc := accounts[idx-1]
		c, err := client.NewClientFromConfig(acc.UserName)
		if err != nil {
			fmt.Printf("加载账号 '%s' 失败: %v\n", acc.UserName, err)
			continue
		}
		fmt.Printf("已加载账号: %s\n", acc.UserName)
		return c
	}
}
