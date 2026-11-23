package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultLoggerFactory(t *testing.T) {
	if logger := defaultRuntimeDependencies.newLogger(); logger == nil {
		t.Fatal("default logger factory returned nil")
	}
}

func TestLogFilePath(t *testing.T) {
	fixedTime := time.Date(2026, time.November, 22, 12, 34, 56, 789000000, time.UTC)
	want := filepath.Join("/logs", "fakessh-2026-11-22-12-34-56-000.log")
	if got := logFilePath("/logs", fixedTime); got != want {
		t.Errorf("logFilePath() = %q, want %q", got, want)
	}
}
