package services

import (
	"context"
	"strings"
	"testing"
)

// Create must reject a missing title before touching any account or API
// service, so a nil-dependency service is enough to exercise validation.
func TestTasksCreate_MissingTitle(t *testing.T) {
	s := NewTasksService(nil, nil)
	res, err := s.Create(context.Background(), calReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing title")
	}
	if !strings.Contains(firstText(res), "title is required") {
		t.Errorf("expected 'title is required', got: %s", firstText(res))
	}
}

func TestTasksComplete_MissingTaskId(t *testing.T) {
	s := NewTasksService(nil, nil)
	res, err := s.Complete(context.Background(), calReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing taskId")
	}
	if !strings.Contains(firstText(res), "taskId is required") {
		t.Errorf("expected 'taskId is required', got: %s", firstText(res))
	}
}

func TestTasksDelete_MissingTaskId(t *testing.T) {
	s := NewTasksService(nil, nil)
	res, err := s.Delete(context.Background(), calReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing taskId")
	}
	if !strings.Contains(firstText(res), "taskId is required") {
		t.Errorf("expected 'taskId is required', got: %s", firstText(res))
	}
}

// tasklistArg defaults to "@default" and honors an explicit non-empty value.
func TestTasklistArg_Default(t *testing.T) {
	if got := tasklistArg(map[string]any{}); got != "@default" {
		t.Errorf("expected '@default' for empty args, got %q", got)
	}
	if got := tasklistArg(map[string]any{"tasklist": ""}); got != "@default" {
		t.Errorf("expected '@default' for empty tasklist, got %q", got)
	}
	if got := tasklistArg(map[string]any{"tasklist": "MyList123"}); got != "MyList123" {
		t.Errorf("expected 'MyList123', got %q", got)
	}
}
