// tcpecho-client: 跨平台(Windows/Linux)TCP echo 客户端。
// 每隔 300ms 向服务端发送一段随机数据,校验回显是否一致,
// 并把连接、断开(含具体原因)、重连、丢包等完整信息记录到日志文件。
package main

import (
	"flag"
	"math/rand"
	"net"
	"time"

	"tcpecho/internal/applog"
	"tcpecho/internal/neterr"
)

const sendInterval = 300 * time.Millisecond

func main() {
	addr := flag.String("addr", "127.0.0.1:9000", "服务端地址,例如 192.168.1.10:9000")
	logPath := flag.String("log", "client.log", "日志文件路径")
	verbose := flag.Bool("verbose", false, "记录每一个包的收发详情(默认只记录异常事件和周期统计,避免日志过大)")
	timeout := flag.Duration("timeout", 3*time.Second, "读/写超时时间,超过该时间未完成读写则判定网络不通")
	statsPeriod := flag.Duration("stats-interval", 30*time.Second, "统计信息打印间隔")
	flag.Parse()

	logger, closeLog := applog.New(*logPath)
	defer closeLog()

	logger.Printf("客户端启动 目标服务端=%s 日志文件=%s 发送间隔=%s 读写超时=%s", *addr, *logPath, sendInterval, *timeout)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	backoff := time.Second
	const maxBackoff = 30 * time.Second
	var lastDisconnectAt time.Time
	firstAttempt := true

	for {
		conn, err := net.DialTimeout("tcp", *addr, *timeout)
		if err != nil {
			logger.Printf("连接失败 目标=%s err=%v 原因=%s 下次重试等待=%s", *addr, err, neterr.Classify(err), backoff)
			time.Sleep(backoff)
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}

		if firstAttempt {
			logger.Printf("已连接到服务端 目标=%s 本地地址=%s", *addr, conn.LocalAddr())
		} else {
			outage := time.Since(lastDisconnectAt)
			logger.Printf("网络已恢复,重新连接成功 目标=%s 本地地址=%s 本次断网持续时间=%s", *addr, conn.LocalAddr(), outage)
		}
		firstAttempt = false
		backoff = time.Second

		connectedAt := time.Now()
		stats := runSession(conn, logger, rng, *verbose, *timeout, *statsPeriod)
		disconnectedAt := time.Now()
		lastDisconnectAt = disconnectedAt

		logger.Printf("连接断开 目标=%s 连接时间=%s 断开时间=%s 持续时长=%s 已发送=%d 已回显=%d 未回显=%d 数据不匹配=%d 断开原因=%s (err=%v)",
			*addr,
			connectedAt.Format("2006-01-02 15:04:05.000"),
			disconnectedAt.Format("2006-01-02 15:04:05.000"),
			disconnectedAt.Sub(connectedAt),
			stats.sent, stats.recv, stats.sent-stats.recv, stats.mismatch,
			neterr.Classify(stats.err), stats.err)

		time.Sleep(backoff)
		backoff = nextBackoff(backoff, maxBackoff)
	}
}

func nextBackoff(cur, max time.Duration) time.Duration {
	cur *= 2
	if cur > max {
		return max
	}
	return cur
}
