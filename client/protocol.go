package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"time"
)

// 帧格式: [4字节长度][8字节seq][8字节发送时间UnixNano][随机payload]
// 长度字段只描述 seq+sentAt+payload 的字节数,不含长度字段自身。
const (
	frameHeaderLen = 8 + 8 // seq + sentAt
	minPayload     = 8
	maxPayload     = 32
)

func encodeFrame(seq uint64, sentAt time.Time, payload []byte) []byte {
	bodyLen := frameHeaderLen + len(payload)
	frame := make([]byte, 4+bodyLen)
	binary.BigEndian.PutUint32(frame[0:4], uint32(bodyLen))
	binary.BigEndian.PutUint64(frame[4:12], seq)
	binary.BigEndian.PutUint64(frame[12:20], uint64(sentAt.UnixNano()))
	copy(frame[20:], payload)
	return frame
}

func decodeFrame(r *bufio.Reader) (seq uint64, sentAt time.Time, payload []byte, err error) {
	var lenBuf [4]byte
	if _, err = io.ReadFull(r, lenBuf[:]); err != nil {
		return
	}
	bodyLen := binary.BigEndian.Uint32(lenBuf[:])
	if bodyLen < frameHeaderLen || bodyLen > frameHeaderLen+maxPayload {
		err = fmt.Errorf("收到非法帧长度: %d,数据可能已损坏", bodyLen)
		return
	}
	body := make([]byte, bodyLen)
	if _, err = io.ReadFull(r, body); err != nil {
		return
	}
	seq = binary.BigEndian.Uint64(body[0:8])
	sentAt = time.Unix(0, int64(binary.BigEndian.Uint64(body[8:16])))
	payload = body[frameHeaderLen:]
	return
}

func randomPayload(rng *rand.Rand) []byte {
	n := minPayload + rng.Intn(maxPayload-minPayload+1)
	buf := make([]byte, n)
	rng.Read(buf)
	return buf
}
