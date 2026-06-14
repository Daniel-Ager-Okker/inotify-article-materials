package main

import (
	"fmt"
	"inotify-project/app"
	"os"
)

func main() {
	// 1.Get test file from cmd args
	if len(os.Args) != 2 {
		fmt.Println("Usage: <executable> <path>")
		os.Exit(1)
	}
	testFilePath := os.Args[1]

	// 2.Run application
	app.Run(testFilePath)
}
