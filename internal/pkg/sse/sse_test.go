package sse

import (
	"net/http/httptest"
	"strings"
	"testing"

	pb "ai-rag-demo/api/nocli/v1"
)

func TestStreamEmitter_EnumAsString(t *testing.T) {
	w := httptest.NewRecorder()
	emitter, err := NewStreamEmitter(w)
	if err != nil {
		t.Fatalf("failed to create emitter: %v", err)
	}

	chunk := &pb.StreamChunk{
		Event:     pb.StreamEventType_SET_ERROR,
		SessionId: "649b0c1e-9fa1-43a8-9309-e129f369ab04",
		Status:    pb.SessionStatus_SS_IDLE,
		Error: &pb.StreamError{
			Code:    500,
			Message: "test error",
		},
	}

	emitter(chunk)

	body := w.Body.String()
	if !strings.Contains(body, `"event":"SET_ERROR"`) {
		t.Errorf("expected string enum SET_ERROR, got: %s", body)
	}
	if !strings.Contains(body, `"status":"SS_IDLE"`) {
		t.Errorf("expected string enum SS_IDLE, got: %s", body)
	}
	if !strings.Contains(body, `"session_id":"649b0c1e-9fa1-43a8-9309-e129f369ab04"`) {
		t.Errorf("expected snake_case field session_id, got: %s", body)
	}
}
