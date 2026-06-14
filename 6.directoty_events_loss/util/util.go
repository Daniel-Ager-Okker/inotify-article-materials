package util

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const DirEventsMask = unix.IN_MODIFY | unix.IN_ATTRIB | unix.IN_DELETE_SELF |
	unix.IN_MOVE_SELF | unix.IN_CREATE | unix.IN_DELETE |
	unix.IN_MOVED_FROM | unix.IN_MOVED_TO

var pathToWatch = make(map[string]int)

// Fixed-size part of C struct inotify_event (name[] follows, variable length).
type inotifyEvent struct {
	Wd     int32
	Mask   uint32
	Cookie uint32
	Len    uint32
}

// Add path to the inotify group recursively
func AddPathToWatchRecursively(inFd int, root string, watchPaths map[int]string) error {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsPermission(walkErr) {
				return filepath.SkipDir
			}
			return walkErr
		}

		wd, err := unix.InotifyAddWatch(inFd, path, uint32(DirEventsMask))
		if err != nil {
			if os.IsPermission(err) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return fmt.Errorf("inotify_add_watch %s: %w", path, err)
		}

		watchPaths[wd] = path
		pathToWatch[path] = wd
		return nil
	})
	if err != nil {
		return fmt.Errorf("adding watches under %s: %w (watches added: %d)", root, err, len(watchPaths))
	}

	return nil
}

// Listen and print to log inotify events
func ListenInotifyEvents(watchPaths map[int]string, ctx context.Context, inFd int) error {
	// 1.Listen indefinitely
	buf := make([]byte, 4096)
	pfds := []unix.PollFd{{Fd: int32(inFd), Events: unix.POLLIN}}

	for ctx.Err() == nil {
		// 2.Use a finite timeout to allow checking ctx cancellation.
		_, err := unix.Poll(pfds, 500)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return fmt.Errorf("poll: %w", err)
		}
		if pfds[0].Revents&unix.POLLIN == 0 {
			continue
		}

		// 3.Read binary data from inotify queue
		n, err := unix.Read(inFd, buf)
		if err != nil {
			if err == unix.EAGAIN || err == unix.EINTR {
				continue
			}
			return fmt.Errorf("read: %w", err)
		}
		if n <= 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// 4.Read binary buffer
		if err := readInotifyBuffer(inFd, watchPaths, buf, n); err != nil {
			return err
		}
	}

	fmt.Printf("Program is over, length of inner storage: %d\n", len(pathToWatch))
	return ctx.Err()
}

// Parse inotify binary buffer
func readInotifyBuffer(inFd int, watchPaths map[int]string, buf []byte, n int) error {
	r := bytes.NewReader(buf[:n])

	for r.Len() > 0 {
		var event inotifyEvent
		if err := binary.Read(r, binary.LittleEndian, &event); err != nil {
			return fmt.Errorf("read event: %w", err)
		}

		name, err := readEventName(r, event.Len)
		if err != nil {
			return err
		}

		path, err := watchPath(watchPaths, int(event.Wd))
		if err != nil {
			continue
		}
		if name == "" {
			continue
		}

		childPath := filepath.Join(path, name)

		switch {
		case event.Mask&(unix.IN_CREATE|unix.IN_MOVED_TO) != 0:
			childWd, err := unix.InotifyAddWatch(inFd, childPath, uint32(DirEventsMask))
			if err != nil {
				fmt.Printf("inotify_add_watch %s: %v\n", childPath, err)
				continue
			}
			watchPaths[childWd] = childPath
			pathToWatch[childPath] = childWd

		case event.Mask&(unix.IN_DELETE|unix.IN_MOVED_FROM) != 0:
			if childWd, ok := pathToWatch[childPath]; ok {
				delete(watchPaths, childWd)
				delete(pathToWatch, childPath)
			}
		}
	}

	return nil
}

func readEventName(r *bytes.Reader, length uint32) (string, error) {
	if length == 0 {
		return "", nil
	}

	nameBytes := make([]byte, length)
	if _, err := io.ReadFull(r, nameBytes); err != nil {
		return "", fmt.Errorf("read name: %w", err)
	}

	return strings.TrimRight(string(nameBytes), "\x00"), nil
}

// Get watching path by watch descriptor
func watchPath(watchPaths map[int]string, wd int) (string, error) {
	path, ok := watchPaths[wd]
	if !ok {
		return "", fmt.Errorf("no path for wd %d", wd)
	}
	return path, nil
}
