package app

import (
	"context"
	"fmt"
	"inotify-project/util"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

var (
	watchPaths = make(map[int]string)
)

func Run(testPath *string) {
	// 1.Create inotify group
	inFd, err := initInotifyGroup()
	if err != nil {
		return
	}
	defer unix.Close(inFd)

	// 2.Add input path to the group with recursion logic
	err = util.AddPathToWatchRecursively(inFd, testPath, watchPaths)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 3.Listen for incoming events forever (until Ctrl+C / SIGTERM).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Mount reference for open_by_handle_at(2): must be an open file on the *same filesystem*
	// as the inode in the FID event; "/" is often a different FS than e.g. /mnt/c or tmpfs.
	if err := util.ListenInotifyEvents(watchPaths, ctx, inFd); err != nil {
		fmt.Printf("listen error: %v\n", err)
		return
	}
}

// Create inotify group
func initInotifyGroup() (int, error) {
	inFd, err := unix.InotifyInit()
	if err != nil {
		fmt.Printf("Error while creating inotify group: %v\n", err)
		return 0, err
	}
	return inFd, nil
}
