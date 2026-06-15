//go:build !windows

package main

import "os"

func maximizeTerminal() {
	os.Stdout.Write([]byte("\033[9;1t"))
}
