// tcpecho-server: 一个简单的 TCP echo 服务端,原样返回收到的数据。
// 仅支持 Linux/Unix 部署运行(依赖标准 net 包,理论上也能在其他平台跑,
// 但按需求只在 Linux 上使用)。
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"tcpecho/internal/applog"
	"tcpecho/internal/neterr"
)

const idleTimeout = 90 * time.Second // 超过此时长没有任何数据,主动断开该连接

var connSeq int64

func main() {
	addr := flag.String("addr", ":9000", "监听地址,例如 :9000 或 0.0.0.0:9000")
	logPath := flag.String("log", "server.log", "日志文件路径")
	flag.Parse()

	logger, closeLog := applog.New(*logPath)
	defer closeLog()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		logger.Fatalf("监听失败 addr=%s err=%v", *addr, err)
	}
	logger.Printf("服务已启动,监听地址=%s", *addr)

	// 收到 Ctrl+C / kill 信号时优雅退出并记录日志
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-sigCh
		logger.Printf("收到信号 %v,服务准备退出", s)
		ln.Close()
		os.Exit(0)
	}()

	var wg sync.WaitGroup
	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				logger.Printf("监听已关闭,停止接受新连接")
				break
			}
			logger.Printf("accept 出错: %v 原因=%s", err, neterr.Classify(err))
			continue
		}
		id := atomic.AddInt64(&connSeq, 1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			handleConn(id, conn, logger)
		}()
	}
	wg.Wait()
}

func handleConn(id int64, conn net.Conn, logger *log.Logger) {
	remote := conn.RemoteAddr().String()
	start := time.Now()
	logger.Printf("[conn#%d] 客户端已连接 remote=%s local=%s", id, remote, conn.LocalAddr())

	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(30 * time.Second)
		_ = tc.SetNoDelay(true)
	}

	defer conn.Close()

	buf := make([]byte, 4096)
	var totalBytes, totalReads int64

	for {
		if err := conn.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
			logger.Printf("[conn#%d] 设置读超时失败: %v", id, err)
		}

		n, rerr := conn.Read(buf)
		if n > 0 {
			totalBytes += int64(n)
			totalReads++
			// 原样返回收到的数据
			if _, werr := conn.Write(buf[:n]); werr != nil {
				logDisconnect(logger, id, remote, start, werr, totalBytes, totalReads)
				return
			}
		}
		if rerr != nil {
			logDisconnect(logger, id, remote, start, rerr, totalBytes, totalReads)
			return
		}
	}
}

func logDisconnect(logger *log.Logger, id int64, remote string, start time.Time, err error, totalBytes, totalReads int64) {
	now := time.Now()
	logger.Printf("[conn#%d] 客户端断开 remote=%s 连接时间=%s 断开时间=%s 持续时长=%s 累计字节=%d 累计包数=%d 断开原因=%s (err=%v)",
		id, remote,
		start.Format("2006-01-02 15:04:05.000"),
		now.Format("2006-01-02 15:04:05.000"),
		now.Sub(start),
		totalBytes, totalReads,
		neterr.Classify(err), err)
}
