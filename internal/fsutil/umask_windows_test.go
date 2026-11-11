//go:build windows

package fsutil

func setUmask(int) (restore func()) { return func() {} }
