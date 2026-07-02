#!/usr/bin/env bash
# 一键交叉编译: 服务端(Linux) + 客户端(Linux/Windows)
set -euo pipefail
cd "$(dirname "$0")"

mkdir -p build

# CGO_ENABLED=0 生成纯静态二进制,不依赖目标机器的 glibc 版本,
# 避免出现 "GLIBC_2.xx not found" 之类的兼容性问题。
export CGO_ENABLED=0

echo "building server (linux/amd64) ..."
GOOS=linux GOARCH=amd64 go build -o build/tcpecho-server-linux-amd64 ./server

echo "building client (linux/amd64) ..."
GOOS=linux GOARCH=amd64 go build -o build/tcpecho-client-linux-amd64 ./client

echo "building client (windows/amd64) ..."
GOOS=windows GOARCH=amd64 go build -o build/tcpecho-client-windows-amd64.exe ./client

echo "building client (windows/386) ..."
GOOS=windows GOARCH=386 go build -o build/tcpecho-client-windows-386.exe ./client

echo "done, artifacts in ./build:"
ls -la build/
