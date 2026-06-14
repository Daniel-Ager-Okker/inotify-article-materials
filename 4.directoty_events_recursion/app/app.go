package app

import (
	"context"
	"fmt"
	"inotify-project/util"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

func Run(path string) {
	// 1.Create inotify group
	inFd, err := unix.InotifyInit()
	if err != nil {
		fmt.Printf("inotify_init: %v\n", err)
		return
	}
	defer unix.Close(inFd)

	// 2.Add input path to the group with recursion logic
	watchPaths := make(map[int]string)
	if err := util.AddPathToWatchRecursively(inFd, path, watchPaths); err != nil {
		fmt.Println(err)
		return
	}

	// 3.Listen for incoming events forever (until Ctrl+C / SIGTERM).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := util.ListenInotifyEvents(watchPaths, ctx, inFd); err != nil && err != context.Canceled {
		fmt.Printf("listen: %v\n", err)
	}
}
