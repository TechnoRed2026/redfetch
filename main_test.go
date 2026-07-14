package main

import (
	"strings"
	"testing"
)

func TestFallback(t *testing.T) {
	if got := fallback("  ", "x"); got != "x" {
		t.Fatalf("fallback empty = %q", got)
	}
	if got := fallback(" y ", "x"); got != "y" {
		t.Fatalf("fallback trim = %q", got)
	}
}

func TestRender(t *testing.T) {
	out := render(info{user: "u", host: "h", os: "o", arch: "a"}, false)
	for _, want := range []string{"u@h", "os:", "arch:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q in %q", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatal("render(false) must not colorize")
	}
}
