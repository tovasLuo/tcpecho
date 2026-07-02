// Package neterr 把 Go 的网络错误归类为便于人读的中文原因,
// 用来在日志中记录“为什么断开了”,而不仅仅是原始 error 字符串。
// 这里用到的 syscall 错误码(ECONNRESET/EPIPE/ETIMEDOUT 等)在
// Go 的 syscall 包里 Windows 和 Linux 均有对应定义,因此可以跨平台编译。
package neterr

import (
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
)

// Classify 返回 err 对应的中文可读原因。
func Classify(err error) string {
	if err == nil {
		return "无错误"
	}

	switch {
	case errors.Is(err, io.EOF):
		return "对端主动关闭了连接(EOF),可能是对端进程退出或正常关闭"
	case errors.Is(err, os.ErrDeadlineExceeded):
		return "读写超时,在设定时间内没有收到数据,判定为网络不通或对端无响应"
	case errors.Is(err, syscall.ECONNRESET):
		return "连接被对端重置(RST),可能是对端异常关闭、防火墙拦截或网络设备中断了连接"
	case errors.Is(err, syscall.EPIPE):
		return "写入已关闭的连接(broken pipe),对端已经断开"
	case errors.Is(err, syscall.ECONNABORTED):
		return "连接被本地协议栈中止"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "连接被拒绝,目标端口没有服务在监听,或被防火墙拒绝"
	case errors.Is(err, syscall.ETIMEDOUT):
		return "连接超时,网络不可达或对端长时间无响应"
	case errors.Is(err, syscall.EHOSTUNREACH):
		return "目标主机不可达,可能路由中断或对端已离线"
	case errors.Is(err, syscall.ENETUNREACH):
		return "网络不可达,本地网络故障或路由丢失"
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "网络操作超时,判定为网络不通"
	}

	msg := err.Error()
	if strings.Contains(msg, "use of closed network connection") {
		return "本地主动关闭了连接"
	}
	return "未分类错误: " + msg
}
