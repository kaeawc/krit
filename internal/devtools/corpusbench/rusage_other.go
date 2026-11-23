//go:build !unix

package main

import "os"

// maxRSSMB is not measured on platforms without getrusage.
func maxRSSMB(*os.ProcessState) float64 { return 0 }
