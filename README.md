<div align="center">

# 🚀 [123pan-cli](https://www.123panng.top)

  <p>突破限制 · 高效下载 · 简单易用</p>
  
  <div>
    <a href="https://github.com/123pannextgen/123pan-cli/stargazers"><img src="https://img.shields.io/github/stars/123pannextgen/123pan-cli" alt="Stars"></a>
    <a href="https://github.com/123pannextgen/123pan-cli/issues"><img src="https://img.shields.io/github/issues/123pannextgen/123pan-cli"></a>
    <a href="https://github.com/123pannextgen/123pan-cli/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-green" alt="License"></a>
    <a href="https://go.dev/dl/"><img src="https://img.shields.io/badge/go-1.22%2B-blue" alt="Go Version"></a>
    <a href="https://github.com/123pannextgen/123pan-cli/releases"><img src="https://img.shields.io/github/v/tag/123pannextgen/123pan-cli?label=release" alt="latest_release"></a>
    <a href="https://github.com/123pannextgen/123pan-cli/releases"><img src="https://img.shields.io/github/downloads/123pannextgen/123pan-cli/total" alt="Downloads"></a>
  </div>
  <br>

</div>

## 介绍

123pan-cli是一款基于Go开发的高效云盘管理工具，通过模拟安卓客户端协议，帮助用户绕过123云盘的自用下载流量限制，实现无阻碍下载体验。支持 **CLI 命令行** 与 **TUI 终端交互界面** 两种模式，体积更小、功能更全。

### 功能特性

- 📂 文件浏览与管理（列表、进入文件夹、返回上级）
- 📥 文件下载（支持多线程分片下载）
- 📤 文件上传（支持分块上传与秒传）
- 🔗 获取文件直链 / 分享文件
- 📁 创建文件夹 / 重命名 / 删除文件
- 🗑️ 回收站查看与恢复
- 🎨 TUI 终端交互界面（基于 Bubble Tea）
- 💾 账户配置持久化（自动保存登录状态）
- 📦 单二进制文件，无需依赖


## 使用

### 快速开始

```shell
# CLI 命令行模式
./123pan-cli -u 你的账号 -p 你的密码

# TUI 终端交互模式
./123pan-cli --tui

# 查看帮助
./123pan-cli --help
```

### CLI 命令

| 命令 | 说明 |
|---|---|
| `list` / `ls` | 列出当前目录文件 |
| `cd <编号>` | 进入文件夹 |
| `cd ..` | 返回上一级 |
| `cd /` | 回到根目录 |
| `pwd` | 显示当前路径 |
| `link <编号>` | 获取文件下载直链 |
| `download/dl <编号> [目录]` | 下载文件 |
| `upload/up <路径>` | 上传文件 |
| `mkdir <名称>` | 创建文件夹 |
| `rename/rn <编号> <新名>` | 重命名文件 |
| `delete/rm <编号>` | 删除文件（移入回收站） |
| `share <编号> [密码]` | 分享文件 |
| `recycle/trash` | 查看回收站 |
| `restore <编号>` | 从回收站恢复文件 |
| `logout` | 退出登录 |
| `help` | 显示帮助 |
| `exit` / `quit` | 退出程序 |

### TUI 快捷键

| 按键 | 功能 |
|---|---|
| `↑↓` / `j``k` | 移动光标 |
| `Enter` | 进入文件夹 / 下载文件 |
| `Backspace` | 返回上级目录 |
| `g` | 回到根目录 |
| `n` | 新建文件夹 |
| `r` | 重命名 |
| `d` | 删除 |
| `s` | 分享 |
| `u` | 上传文件 |
| `l` | 登录 / 切换账号 |
| `Ctrl+R` | 刷新列表 |
| `q` | 退出 |

### 使用打包后的文件运行

如果你的电脑是windows系统或者linux发行版，可以直接下载打包后的文件并运行。  
下载地址：

- Github: https://github.com/123panNextGen/123pan-cli/releases/
- Website（Cloudflare CDN，更新可能不及时）：https://download.123panng.top/123pan-cli/

>[!TIP]
>Windows下如果无法运行，可以尝试打开兼容模式。杀毒软件有可能报毒，请放行。

>[!IMPORTANT]
>请不要从未知渠道下载！

### 使用源码编译

首先准备好 [Go](https://go.dev/dl/) 1.22+ 环境，并克隆存储库。

```shell
git clone https://github.com/123panNextGen/123pan-cli.git
cd 123pan-cli/
```

然后编译并运行：

```shell
# 直接编译
go build -o 123pan-cli ./cmd/123pan-cli/

# 使用构建脚本
chmod +x scripts/build.sh && ./scripts/build.sh

# 运行
./123pan-cli
```

## 问题反馈

你可以通过多种途径反馈问题。

- Github: https://github.com/123panNextGen/123pan-cli/issues
- QQ群: 996241397

我们将在第一时间解决。

## 社区讨论：

你可以在社区讨论相关问题

- Github：https://github.com/123panNextGen/123pan-cli/discussions
- QQ群：群号同上

## 使用协议

本程序使用[MIT](./LICENSE)协议。  

## 免责声明

本项目为**个人学习与技术研究目的开发，与 123 云盘官方无任何关联。**使用本软件即表示您已**知晓并同意**以下内容：

- **本软件按「现状」提供，不提供任何明示或暗示的保证**
- **开发者不对因使用本软件导致的任何直接或间接损失承担责任，包括但不限于数据丢失、账号封禁、服务中断等**
- **使用者应自行承担使用本软件的全部风险，并遵守 123 云盘用户协议及相关法律法规**
- **请勿将本软件用于商业用途**