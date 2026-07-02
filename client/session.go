package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type pendingItem struct {
	seq     uint64
	sentAt  time.Time
	payload []byte
}

type sessionStats struct {
	sent, recv, mismatch uint64
	err                  error
}

// runSession 在一个已建立的连接上运行发送/接收循环,直到连接出现问题为止,
// 返回本次会话的统计信息和导致断开的错误。
func runSession(conn net.Conn, logger *log.Logger, rng *rand.Rand, verbose bool, timeout, statsPeriod time.Duration) sessionStats {
	defer conn.Close()

	var mu sync.Mutex
	pending := make([]pendingItem, 0, 64)

	errCh := make(chan error, 2)
	reportErr := func(err error) {
		select {
		case errCh <- err:
		default:
		}
	}

	var seq, sentCount, recvCount, mismatchCount uint64

	var wg sync.WaitGroup
	wg.Add(2)

	// 发送 goroutine: 每 300ms 生成一段随机数据并发送
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(sendInterval)
		defer ticker.Stop()
		for range ticker.C {
			s := atomic.AddUint64(&seq, 1)
			payload := randomPayload(rng)
			sentAt := time.Now()
			frame := encodeFrame(s, sentAt, payload)

			mu.Lock()
			pending = append(pending, pendingItem{seq: s, sentAt: sentAt, payload: payload})
			mu.Unlock()

			if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
				reportErr(fmt.Errorf("设置写超时失败: %w", err))
				return
			}
			if _, err := conn.Write(frame); err != nil {
				reportErr(fmt.Errorf("发送数据失败: %w", err))
				return
			}
			atomic.AddUint64(&sentCount, 1)
			if verbose {
				logger.Printf("发送 seq=%d 时间=%s 长度=%d", s, sentAt.Format("15:04:05.000"), len(payload))
			}
		}
	}()

	// 接收 goroutine: 读取回显并校验数据是否原样返回
	go func() {
		defer wg.Done()
		r := bufio.NewReader(conn)
		lastStatsLog := time.Now()
		for {
			if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
				reportErr(fmt.Errorf("设置读超时失败: %w", err))
				return
			}
			gotSeq, gotSentAt, gotPayload, err := decodeFrame(r)
			if err != nil {
				reportErr(fmt.Errorf("接收回显失败: %w", err))
				return
			}

			mu.Lock()
			matched := false
			var sentPayload []byte
			for i, p := range pending {
				if p.seq == gotSeq {
					sentPayload = p.payload
					pending = append(pending[:i], pending[i+1:]...)
					matched = true
					break
				}
			}
			mu.Unlock()

			if matched && !bytes.Equal(sentPayload, gotPayload) {
				atomic.AddUint64(&mismatchCount, 1)
				logger.Printf("警告: seq=%d 回显数据与发送数据不一致,疑似网络数据损坏", gotSeq)
			}

			atomic.AddUint64(&recvCount, 1)
			rtt := time.Since(gotSentAt)
			if verbose {
				logger.Printf("回显 seq=%d rtt=%s matched=%v", gotSeq, rtt, matched)
			}

			if time.Since(lastStatsLog) >= statsPeriod {
				logger.Printf("连接正常,统计: 已发送=%d 已回显=%d 最近RTT=%s",
					atomic.LoadUint64(&sentCount), atomic.LoadUint64(&recvCount), rtt)
				lastStatsLog = time.Now()
			}
		}
	}()

	err := <-errCh
	conn.Close() // 解除另一个 goroutine 在 Read/Write 上的阻塞
	wg.Wait()

	return sessionStats{
		sent:     atomic.LoadUint64(&sentCount),
		recv:     atomic.LoadUint64(&recvCount),
		mismatch: atomic.LoadUint64(&mismatchCount),
		err:      err,
	}
}
