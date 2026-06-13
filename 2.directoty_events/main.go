package main

import (
	"fmt"
	"inotify-project/app"
	"os"
)

func main() {
	// 1.Get test file from cmd args
	argsCnt := len(os.Args)
	if argsCnt != 2 {
		fmt.Println("Usage: <executable> <test-file-path>")
		return
	}
	testFilePath := os.Args[1]

	// 2.Run application
	app.Run(&testFilePath)
}
