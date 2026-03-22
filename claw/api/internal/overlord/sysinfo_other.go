//go:build !linux

package overlord

// cpuPercent returns 0 on non-Linux platforms (dev fallback).
func cpuPercent() float64 { return 0 }

// memPercent returns 0 on non-Linux platforms (dev fallback).
func memPercent() float64 { return 0 }
