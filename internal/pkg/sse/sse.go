package sse

import (
	"fmt"
	"net/http"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli"

	"google.golang.org/protobuf/encoding/protojson"
)

var protoMarshaler = protojson.MarshalOptions{
	UseProtoNames: true,
}

// SetHeaders 自动设置符合标准的 HTTP SSE 响应头
func SetHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Nginx 禁用反向代理响应缓冲，保证即时打字机输出
}

// NewStreamEmitter 创建通用且高复用的 SSE 流式推送闭包
// 自动完成：1. Headers 设置 2. Flusher 断言 3. Protobuf/JSON 序列化 4. 实时 Flush 吐给 Socket
func NewStreamEmitter(w http.ResponseWriter) (nocli.StreamEmitter, error) {
	SetHeaders(w)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported: response writer does not implement http.Flusher")
	}

	return func(chunk *pb.StreamChunk) {
		if chunk == nil {
			return
		}

		jsonData, err := protoMarshaler.Marshal(chunk)
		if err != nil {
			return
		}

		// 格式化输出标准的 SSE 帧结构:
		// event: <event_name>\n
		// data: <json_string>\n\n
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", chunk.Event.String(), jsonData)
		flusher.Flush()
	}, nil
}
