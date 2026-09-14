package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestOutputSetWriterAndJSON(t *testing.T) {
	var buf bytes.Buffer
	SetWriter(&buf)
	defer ResetWriter()

	dto := ListSummaryDTO{
		Name:     "test-list",
		Total:    42,
		Disabled: 2,
	}

	if err := JSON(dto); err != nil {
		t.Fatalf("JSON failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `"name": "test-list"`) {
		t.Errorf("expected json to contain name test-list, got: %s", got)
	}
	if !strings.Contains(got, `"total": 42`) {
		t.Errorf("expected json to contain total 42, got: %s", got)
	}
}

func TestOutputBannerAndRow(t *testing.T) {
	var buf bytes.Buffer
	SetWriter(&buf)
	defer ResetWriter()

	Info("info message")
	Warn("warn message")
	ListRow("list1", 10, 1)

	got := buf.String()
	if !strings.Contains(got, "info message") {
		t.Errorf("expected output to contain info message, got: %s", got)
	}
	if !strings.Contains(got, "warn message") {
		t.Errorf("expected output to contain warn message, got: %s", got)
	}
	if !strings.Contains(got, "list1") {
		t.Errorf("expected output to contain list1, got: %s", got)
	}
}
