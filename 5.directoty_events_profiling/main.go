package main

import (
	"fmt"
	"inotify-project/app"
	"log"
	"os"
	"runtime/pprof"
)

func main() {
	// 1.CPU profiling
	const cpuprofile = "cpu.out"
	f, err := os.Create(cpuprofile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	// 2.Get test file from cmd args
	argsCnt := len(os.Args)
	if argsCnt != 2 {
		fmt.Println("Usage: <executable> <test-file-path>")
		return
	}
	testFilePath := os.Args[1]

	// 3.Run application
	app.Run(&testFilePath)
}
