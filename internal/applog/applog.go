// Package applog 提供一个同时写入标准输出和日志文件的简单 logger,
// client 与 server 共用同一套日志格式,方便比对两端日志排查问题。
package applog

import (
	"io"
	"log"
	"os"
)

// New 打开(或创建)path 对应的日志文件,返回一个同时输出到 stdout 和文件的
// *log.Logger,以及一个用于关闭文件的函数。
func New(path string) (*log.Logger, func() error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("无法打开日志文件 %s: %v", path, err)
	}
	mw := io.MultiWriter(os.Stdout, f)
	logger := log.New(mw, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	return logger, f.Close
}
