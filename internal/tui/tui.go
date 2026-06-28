package tui

import (
	"fmt"
	"strings"

	"123pan-cli/internal/client"
	"123pan-cli/internal/model"
	"123pan-cli/internal/utils"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B"))

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4"))

	dirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")).
			Bold(true)

	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))
)

// screen 表示不同界面
type screen int

const (
	screenLogin screen = iota
	screenMain
	screenInput
)

// Model 主 TUI 模型
type Model struct {
	client *client.Client
	screen screen
	width  int
	height int

	// 文件列表
	files    []model.FileItem
	cursor   int
	offset   int
	viewport viewport.Model
	ready    bool

	// 消息
	message     string
	messageType string // "info", "error", "success"

	// 输入
	inputs      []textinput.Model
	inputFocus  int
	inputPrompt string
	inputAction string // 当前输入动作: "login", "rename", "mkdir", "share", "download", "upload"

	// 登录
	loginStep int // 0=输账号, 1=输密码

	// 状态栏
	statusMsg string

	// 任务管理
	tasks []*model.TransferTask

	// 确认框
	confirmMode   bool
	confirmMsg    string
	confirmAction string
	confirmTarget int64

	// 键盘输入缓冲区
	keys string
}

// NewModel 创建新的 TUI Model
func NewModel(c *client.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "输入账号..."
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40

	pw := textinput.New()
	pw.Placeholder = "输入密码..."
	pw.EchoMode = textinput.EchoPassword
	pw.EchoCharacter = '*'
	pw.CharLimit = 64
	pw.Width = 40

	vp := viewport.New(80, 20)

	return Model{
		client:      c,
		screen:      screenMain,
		files:       c.FileList(),
		viewport:    vp,
		inputs:      []textinput.Model{ti, pw},
		cursor:      0,
		offset:      0,
		message:     "欢迎使用 123pan-cli TUI",
		messageType: "info",
		statusMsg:   "按 ? 查看帮助",
	}
}

// Init 初始化
func (m Model) Init() tea.Cmd {
	return nil
}

// Update 处理消息
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-6)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
		}
		return m, nil

	case tea.KeyMsg:
		if m.confirmMode {
			return m.handleConfirmKey(msg)
		}

		switch m.screen {
		case screenLogin:
			return m.handleLoginKey(msg)
		case screenInput:
			return m.handleInputKey(msg)
		case screenMain:
			return m.handleMainKey(msg)
		}
	}

	// 更新输入框
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View 渲染视图
func (m Model) View() string {
	if !m.ready {
		return "初始化中..."
	}

	switch m.screen {
	case screenLogin:
		return m.viewLogin()
	case screenInput:
		return m.viewInput()
	default:
		return m.viewMain()
	}
}

// ----------- 登录界面 -----------

func (m Model) viewLogin() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" 123pan-cli 登录 "))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("账号: %s\n", m.inputs[0].View()))
	if m.loginStep >= 1 {
		b.WriteString(fmt.Sprintf("密码: %s\n", m.inputs[1].View()))
	}

	if m.message != "" {
		style := statusStyle
		if m.messageType == "error" {
			style = errorStyle
		} else if m.messageType == "success" {
			style = successStyle
		}
		b.WriteString("\n" + style.Render(m.message))
	}

	b.WriteString("\n\n" + helpStyle.Render("Tab 切换 | Enter 确认 | Esc 返回"))
	return b.String()
}

func (m Model) handleLoginKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		if m.loginStep < 1 {
			m.loginStep = 1
			m.inputs[1].Focus()
			m.inputs[0].Blur()
		} else {
			m.loginStep = 0
			m.inputs[0].Focus()
			m.inputs[1].Blur()
		}
		return m, nil

	case "enter":
		if m.loginStep == 0 {
			m.loginStep = 1
			m.inputs[1].Focus()
			m.inputs[0].Blur()
			return m, nil
		}
		userName := m.inputs[0].Value()
		password := m.inputs[1].Value()
		if userName == "" || password == "" {
			m.message = "账号或密码不能为空"
			m.messageType = "error"
			return m, nil
		}
		if err := m.client.LoginWithCredentials(userName, password); err != nil {
			m.message = fmt.Sprintf("登录失败: %v", err)
			m.messageType = "error"
			return m, nil
		}
		m.message = "登录成功！"
		m.messageType = "success"
		m.screen = screenMain
		m.client.RefreshFileList()
		m.files = m.client.FileList()
		return m, nil

	case "esc":
		m.screen = screenMain
		return m, nil
	}
	return m, nil
}

// ----------- 主界面 -----------

func (m Model) viewMain() string {
	// 标题栏
	title := titleStyle.Render(fmt.Sprintf(" 123pan-cli | %s ", m.client.CurrentPath()))

	// 文件列表
	m.viewport.SetContent(m.renderFileList())
	fileView := m.viewport.View()

	// 状态栏
	statusBar := m.renderStatusBar()

	// 帮助栏
	helpBar := m.renderHelpBar()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		fileView,
		statusBar,
		helpBar,
	)
}

func (m Model) renderFileList() string {
	if len(m.files) == 0 {
		return "\n  (空目录)\n"
	}

	var b strings.Builder
	for i, f := range m.files {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▶ ")
		}

		icon := "📄"
		style := fileStyle
		if f.IsDir() {
			icon = "📁"
			style = dirStyle
		}

		line := fmt.Sprintf("%s%s %s  %s",
			cursor, icon,
			style.Render(utils.TruncateString(f.FileName, 40)),
			statusStyle.Render(utils.FormatFileSize(f.Size)),
		)
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m Model) renderStatusBar() string {
	left := fmt.Sprintf(" %d/%d 个文件", m.cursor+1, len(m.files))
	right := m.message
	if m.statusMsg != "" {
		right = m.statusMsg
	}

	bar := lipgloss.NewStyle().
		Background(lipgloss.Color("#44475A")).
		Width(m.width).
		Render(left + strings.Repeat(" ", m.width-len(left)-len(right)-2) + right)
	return bar
}

func (m Model) renderHelpBar() string {
	keys := []string{
		"↑↓:移动", "Enter:进入/下载", "Backspace:返回",
		"n:新建文件夹", "r:重命名", "d:删除",
		"s:分享", "u:上传", "l:登录",
		"q:退出", "?:帮助",
	}
	help := strings.Join(keys, " │ ")
	if len(help) > m.width {
		help = help[:m.width]
	}
	return helpStyle.Render(help)
}

func (m Model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.files)-1 {
			m.cursor++
		}

	case "enter":
		if len(m.files) == 0 {
			return m, nil
		}
		file := m.files[m.cursor]
		if file.IsDir() {
			// 进入文件夹
			if err := m.client.ChangeDirByIndex(m.cursor + 1); err != nil {
				m.message = fmt.Sprintf("错误: %v", err)
				m.messageType = "error"
				return m, nil
			}
			m.files = m.client.FileList()
			m.cursor = 0
			m.message = fmt.Sprintf("进入: %s", file.FileName)
			m.messageType = "info"
		} else {
			// 下载文件
			m.message = fmt.Sprintf("正在获取 %s 的下载链接...", file.FileName)
			m.messageType = "info"
			go func() {
				url, err := m.client.GetDownloadLink(file)
				if err != nil {
					m.message = fmt.Sprintf("获取链接失败: %v", err)
					m.messageType = "error"
					return
				}
				path, err := m.client.DownloadFile(url, file.FileName, "downloads")
				if err != nil {
					m.message = fmt.Sprintf("下载失败: %v", err)
					m.messageType = "error"
					return
				}
				m.message = fmt.Sprintf("下载完成: %s", path)
				m.messageType = "success"
			}()
		}

	case "backspace":
		if err := m.client.GoBack(); err != nil {
			m.message = err.Error()
			m.messageType = "error"
			return m, nil
		}
		m.files = m.client.FileList()
		m.cursor = 0
		m.message = "返回上级目录"
		m.messageType = "info"

	case "n":
		// 新建文件夹
		m.screen = screenInput
		m.inputAction = "mkdir"
		m.inputPrompt = "新建文件夹名称:"
		m.inputs[0].SetValue("")
		m.inputs[0].Placeholder = "输入文件夹名..."
		m.inputs[0].Focus()
		m.inputs[0].CharLimit = 128

	case "r":
		// 重命名
		if len(m.files) == 0 || m.cursor >= len(m.files) {
			return m, nil
		}
		m.screen = screenInput
		m.inputAction = "rename"
		m.inputPrompt = fmt.Sprintf("重命名 '%s' 为:", m.files[m.cursor].FileName)
		m.inputs[0].SetValue(m.files[m.cursor].FileName)
		m.inputs[0].Focus()
		m.inputs[0].CharLimit = 128

	case "d":
		// 删除
		if len(m.files) == 0 || m.cursor >= len(m.files) {
			return m, nil
		}
		file := m.files[m.cursor]
		m.confirmMode = true
		m.confirmMsg = fmt.Sprintf("确定要删除 '%s' 吗？(y/n)", file.FileName)
		m.confirmAction = "delete"
		m.confirmTarget = file.FileID

	case "s":
		// 分享
		if len(m.files) == 0 || m.cursor >= len(m.files) {
			return m, nil
		}
		m.screen = screenInput
		m.inputAction = "share"
		m.inputPrompt = "分享密码 (留空则无密码):"
		m.inputs[0].SetValue("")
		m.inputs[0].Placeholder = "可选密码..."
		m.inputs[0].Focus()

	case "u":
		// 上传
		m.screen = screenInput
		m.inputAction = "upload"
		m.inputPrompt = "上传文件路径:"
		m.inputs[0].SetValue("")
		m.inputs[0].Placeholder = "/path/to/file..."
		m.inputs[0].Focus()

	case "l":
		// 登录界面
		m.screen = screenLogin
		m.loginStep = 0
		m.inputs[0].SetValue("")
		m.inputs[1].SetValue("")
		m.inputs[0].Focus()
		m.inputs[1].Blur()
		m.message = ""

	case "g":
		// 回到根目录
		if err := m.client.GoToRoot(); err != nil {
			m.message = err.Error()
			m.messageType = "error"
			return m, nil
		}
		m.files = m.client.FileList()
		m.cursor = 0
		m.message = "已回到根目录"
		m.messageType = "info"

	case "ctrl+r":
		// 刷新
		if err := m.client.RefreshFileList(); err != nil {
			m.message = fmt.Sprintf("刷新失败: %v", err)
			m.messageType = "error"
		} else {
			m.files = m.client.FileList()
			m.cursor = 0
			m.message = "已刷新"
			m.messageType = "info"
		}

	case "?":
		m.showHelp()
	}

	return m, nil
}

func (m *Model) showHelp() {
	m.message = `快捷键:
  ↑↓/jk  移动光标
  Enter   进入文件夹 / 下载文件
  Backspace  返回上级
  g       回到根目录
  n       新建文件夹
  r       重命名
  d       删除
  s       分享
  u       上传文件
  l       登录/切换账号
  Ctrl+R  刷新列表
  q       退出
  ?       显示帮助`
	m.messageType = "info"
}

// ----------- 确认框 -----------

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirmMode = false
		switch m.confirmAction {
		case "delete":
			if err := m.client.DeleteFile(m.confirmTarget); err != nil {
				m.message = fmt.Sprintf("删除失败: %v", err)
				m.messageType = "error"
			} else {
				m.files = m.client.FileList()
				if m.cursor >= len(m.files) {
					m.cursor = len(m.files) - 1
				}
				if m.cursor < 0 {
					m.cursor = 0
				}
				m.message = "已删除"
				m.messageType = "success"
			}
		}
		return m, nil

	case "n", "N", "esc":
		m.confirmMode = false
		m.message = "已取消"
		m.messageType = "info"
		return m, nil
	}
	return m, nil
}

// ----------- 输入界面 -----------

func (m Model) viewInput() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" 输入 "))
	b.WriteString("\n\n")

	if m.confirmMode {
		b.WriteString(m.confirmMsg)
		b.WriteString("\n\n" + helpStyle.Render("y=确认 n=取消"))
		return b.String()
	}

	b.WriteString(m.inputPrompt + "\n")
	b.WriteString(m.inputs[0].View() + "\n")

	if m.message != "" {
		style := statusStyle
		if m.messageType == "error" {
			style = errorStyle
		} else if m.messageType == "success" {
			style = successStyle
		}
		b.WriteString("\n" + style.Render(m.message))
	}

	b.WriteString("\n\n" + helpStyle.Render("Enter 确认 | Esc 取消"))
	return b.String()
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		value := m.inputs[0].Value()
		if value == "" && m.inputAction != "share" {
			m.message = "输入不能为空"
			m.messageType = "error"
			return m, nil
		}
		m.screen = screenMain
		switch m.inputAction {
		case "mkdir":
			id, err := m.client.CreateDir(value)
			if err != nil {
				m.message = fmt.Sprintf("创建失败: %v", err)
				m.messageType = "error"
			} else {
				m.files = m.client.FileList()
				m.message = fmt.Sprintf("文件夹已创建 (ID: %d)", id)
				m.messageType = "success"
			}
		case "rename":
			if len(m.files) == 0 || m.cursor >= len(m.files) {
				return m, nil
			}
			if err := m.client.RenameFile(m.files[m.cursor].FileID, value); err != nil {
				m.message = fmt.Sprintf("重命名失败: %v", err)
				m.messageType = "error"
			} else {
				m.files = m.client.FileList()
				m.message = "重命名成功"
				m.messageType = "success"
			}
		case "share":
			if len(m.files) == 0 || m.cursor >= len(m.files) {
				return m, nil
			}
			url, err := m.client.ShareFileByIndex(m.cursor+1, value)
			if err != nil {
				m.message = fmt.Sprintf("分享失败: %v", err)
				m.messageType = "error"
			} else {
				m.message = fmt.Sprintf("分享链接: %s", url)
				m.messageType = "success"
			}
		case "upload":
			id, err := m.client.UploadFile(value)
			if err != nil {
				m.message = fmt.Sprintf("上传失败: %v", err)
				m.messageType = "error"
			} else {
				m.files = m.client.FileList()
				m.message = fmt.Sprintf("上传成功 (ID: %d)", id)
				m.messageType = "success"
			}
		}
		return m, nil

	case "esc":
		m.screen = screenMain
		m.message = "已取消"
		m.messageType = "info"
		return m, nil
	}
	return m, nil
}

// ----------- 辅助 -----------

// SetFiles 更新文件列表
func (m *Model) SetFiles(files []model.FileItem) {
	m.files = files
	m.cursor = 0
}

// SetMessage 设置消息
func (m *Model) SetMessage(msg, msgType string) {
	m.message = msg
	m.messageType = msgType
}

// ResizeViewport 调整视图大小
func (m *Model) ResizeViewport(w, h int) {
	m.viewport.Width = w
	m.viewport.Height = h - 6
}
