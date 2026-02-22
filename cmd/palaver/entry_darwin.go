//go:build darwin

package main

import "golang.design/x/mainthread"

func main() {
	mainthread.Init(run)
}
