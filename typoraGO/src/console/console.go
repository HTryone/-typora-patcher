package console

// 控制台界面：纯标准库，不依赖颜色库。
// Windows 10/11 的 conhost 与 Windows Terminal 都支持 ANSI 转义；
// 若颜色把界面搞花（旧系统），可用 --no-color 关掉。

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var (
	colorOn   = true
	lineInput = bufio.NewReader(os.Stdin)
)

const (
	cReset  = "\x1b[0m"
	cBold   = "\x1b[1m"
	cDim    = "\x1b[2m"
	cRed    = "\x1b[31m"
	cGreen  = "\x1b[32m"
	cYellow = "\x1b[33m"
	cCyan   = "\x1b[36m"
)

// 导出的颜色常量（调用方拼 Paint 用）。
const (
	ColorRed    = cRed
	ColorGreen  = cGreen
	ColorYellow = cYellow
	ColorCyan   = cCyan
	ColorDim    = cDim
)

// SetColor 开关彩色输出。
func SetColor(on bool) { colorOn = on }

// Paint 给文本上色（关闭彩色时原样返回）。
func Paint(color, s string) string {
	if !colorOn {
		return s
	}
	return color + s + cReset
}

// Banner 打印工具抬头。
func Banner() {
	fmt.Println()
	fmt.Println(Paint(cBold+cCyan, "  ╔══════════════════════════════════════════════╗"))
	fmt.Println(Paint(cBold+cCyan, "  ║        Typora 激活工具  (Go 单文件版)        ║"))
	fmt.Println(Paint(cBold+cCyan, "  ╚══════════════════════════════════════════════╝"))
}

// ReadLine 读一行输入，去掉首尾空白。
// stdin 结束（EOF / Ctrl+Z / 管道耗尽）时直接退出程序，避免菜单空转。
func ReadLine() string {
	s, err := lineInput.ReadString('\n')
	if err != nil && s == "" {
		os.Exit(0)
	}
	return strings.TrimSpace(s)
}

// Ask 提示输入；为空时返回默认值。
func Ask(prompt, def string) string {
	if def != "" {
		fmt.Printf("  %s [%s]: ", prompt, Paint(cDim, def))
	} else {
		fmt.Printf("  %s: ", prompt)
	}
	s := ReadLine()
	if s == "" {
		return def
	}
	return s
}

// Confirm 是/否确认，默认值由 def 决定。
func Confirm(prompt string, def bool) bool {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	fmt.Printf("  %s %s: ", prompt, hint)
	s := strings.ToLower(ReadLine())
	if s == "" {
		return def
	}
	return s == "y" || s == "yes" || s == "是"
}

// Pause 等用户按回车。
func Pause() {
	fmt.Print(Paint(cDim, "  按回车继续…"))
	_, _ = lineInput.ReadString('\n')
}

// Ok / Warn / Fail / Info 打印一条“成功/提示/失败/细节”。
func Ok(msg string)   { fmt.Println("  " + Paint(cGreen, "✓ ") + msg) }
func Warn(msg string) { fmt.Println("  " + Paint(cYellow, "! ") + msg) }
func Fail(msg string) { fmt.Println("  " + Paint(cRed, "✗ ") + msg) }
func Info(msg string) { fmt.Println("  " + Paint(cDim, "· ") + msg) }
