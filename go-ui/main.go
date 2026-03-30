package main

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unsafe"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/sys/windows"

	"netFlow_tool-ui/core"
	"netFlow_tool-ui/ipc"
	"netFlow_tool-ui/service"
	"netFlow_tool-ui/ui"
)

type adminPromptModel struct {
	message string
	detail  string
	err     error
}

func (m adminPromptModel) Init() tea.Cmd { return nil }

func (m adminPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q":
			if err := relaunchAsAdmin(); err != nil {
				m.err = err
				return m, nil
			}
			return m, tea.Quit
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m adminPromptModel) View() string {
	var extra string
	if m.err != nil {
		extra = fmt.Sprintf("\n  提权重启失败：%v\n", m.err)
	}
	return fmt.Sprintf(
		"\n  ⚠  %s\n\n  %s\n%s\n  按 Q 以管理员身份重启，或按 Ctrl+C 退出。\n",
		m.message, m.detail, extra,
	)
}

func showAdminPrompt(msg, detail string) {
	p := tea.NewProgram(adminPromptModel{message: msg, detail: detail}, tea.WithAltScreen())
	_, _ = p.Run()
}

func isAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "access is denied") || strings.Contains(s, "拒绝访问")
}

func isRunningAsAdmin() bool {
	// UAC elevation check: true only when this process is running elevated.
	return windows.GetCurrentProcessToken().IsElevated()
}

func relaunchAsAdmin() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法获取当前程序路径: %w", err)
	}

	exePtr, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return fmt.Errorf("无法解析程序路径: %w", err)
	}

	verbPtr, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return fmt.Errorf("无法解析 runas 动词: %w", err)
	}

	shell32 := windows.NewLazySystemDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")
	if err := shell32.Load(); err != nil {
		return fmt.Errorf("无法加载 shell32.dll: %w", err)
	}

	ret, _, callErr := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(exePtr)),
		0,
		0,
		uintptr(windows.SW_NORMAL),
	)
	if ret <= 32 {
		if callErr != windows.ERROR_SUCCESS && callErr != nil {
			return fmt.Errorf("ShellExecuteW 失败: %w", callErr)
		}
		return fmt.Errorf("ShellExecuteW 返回错误码 %d", ret)
	}

	return nil
}

func main() {
	if !isRunningAsAdmin() {
		showAdminPrompt(
			"检测到当前未以管理员身份运行。",
			"管理员权限仅用于 WinDivert 组件的抓包监测，不涉及其他安全风险。你可以直接按 Q 自动请求提权并重启本程序。",
		)
		os.Exit(1)
	}

	// ── 1. Launch the Rust core (or detect it's already running) ──
	launcher := core.NewLauncher()
	if err := launcher.Start(); err != nil {
		if isAccessDenied(err) {
			showAdminPrompt("当前会话无权限启动 netFlow_tool 核心进程。",
				"请以管理员身份运行本程序（右键→以管理员身份运行）。")
			os.Exit(1)
		}
		showAdminPrompt(
			fmt.Sprintf("无法启动核心进程：%v", err),
			"请确认 netFlow_tool-core.exe 与本程序在同一目录，并以管理员身份运行。",
		)
		os.Exit(1)
	}
	defer launcher.Stop() // UI 退出时自动关闭核心进程

	if pid := launcher.CorePID(); pid > 0 {
		fmt.Printf("核心进程已启动 (PID: %d)，正在连接...\n", pid)
	}

	// ── 2. Connect to the Rust core via named pipe ──
	client, err := ipc.NewClient()
	if err != nil {
		if isAccessDenied(err) {
			showAdminPrompt("当前会话无权限访问 netFlow_tool 核心进程。", "请使用管理员权限重新运行。")
			os.Exit(1)
		}
		showAdminPrompt(
			fmt.Sprintf("无法连接 netFlow_tool 核心进程：%v", err),
			"核心进程可能启动失败，请以管理员身份重新运行。",
		)
		os.Exit(1)
	}
	defer client.Close()

	// Ping to verify connection
	if err := client.Ping(); err != nil {
		if isAccessDenied(err) {
			showAdminPrompt("当前会话无权限访问 netFlow_tool 核心进程。", "请使用管理员权限重新运行。")
			os.Exit(1)
		}
		showAdminPrompt(
			fmt.Sprintf("核心进程无响应：%v", err),
			"请以管理员身份重新运行。",
		)
		os.Exit(1)
	}

	// ── 3. Start polling service & TUI ──
	statsSvc := service.NewStatsService(client, 1*time.Second)
	statsSvc.Start()
	defer statsSvc.Stop()

	model := ui.NewModel(statsSvc)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
