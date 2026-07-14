//go:build !windows

package main

func nativeRAM() string     { return "" }
func nativeUptime() string  { return "" }
func nativeDisks() []string { return nil }
func enableVT() bool        { return true }
