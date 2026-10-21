package main

import (
	"bytes"
	"context"
	"testing"
)

func TestVersionFlagPrintsVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); got != version+"\n" {
		t.Fatalf("stdout = %q, want %q", got, version+"\n")
	}
}

func TestUnknownFlagExitsWithUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-nope"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
