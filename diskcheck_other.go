//go:build windows

package main

// hasDiskSpace is not implemented on Windows. Returns nil (skip check).
func hasDiskSpace(_ string, _ int64) error { return nil }
