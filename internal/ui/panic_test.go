package ui

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLogPanic_WritesStackAndRepanics(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	repanicked := func() (r any) {
		defer func() { r = recover() }()
		func() {
			defer logPanic()
			panic("boom")
		}()
		return nil
	}()
	if repanicked != "boom" {
		t.Errorf("panic should propagate, got %v", repanicked)
	}
	if !strings.Contains(buf.String(), "panic: boom") || !strings.Contains(buf.String(), "goroutine") {
		t.Errorf("log should have the panic and stack, got %q", buf.String())
	}
}
