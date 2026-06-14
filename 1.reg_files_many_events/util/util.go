package util

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const FileEventsMask = unix.IN_MODIFY | unix.IN_ATTRIB | unix.IN_DELETE_SELF | unix.IN_MOVE_SELF

// Fixed-size part of C struct inotify_event (name[] follows, variable length).
type inotifyEvent struct {
	Wd     int32
	Mask   uint32
	Cookie uint32
	Len    uint32
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
		if err := readInotifyBuffer(watchPaths, buf, n); err != nil {
			return err
		}
	}

	return ctx.Err()
}

// Parse inotify binary buffer
func readInotifyBuffer(watchPaths map[int]string, buf []byte, n int) error {
	r := bytes.NewReader(buf[:n])

	for r.Len() > 0 {
		var event inotifyEvent
		if err := binary.Read(r, binary.LittleEndian, &event); err != nil {
			return fmt.Errorf("read event: %w", err)
		}

		if event.Len > 0 {
			if _, err := r.Seek(int64(event.Len), io.SeekCurrent); err != nil {
				return fmt.Errorf("skip name: %w", err)
			}
		}

		path, err := watchPath(watchPaths, int(event.Wd))
		if err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Printf("Got event: wd=%d mask=%s path=%s\n",
			event.Wd, formatEventMask(event.Mask), path)
	}

	return nil
}

// Get event mask in string representation
func formatEventMask(mask uint32) string {
	var parts []string

	if mask&unix.IN_MODIFY != 0 {
		parts = append(parts, "modification")
	}
	if mask&unix.IN_ATTRIB != 0 {
		parts = append(parts, "change attributes")
	}
	if mask&unix.IN_DELETE_SELF != 0 {
		parts = append(parts, "self deletion")
	}
	if mask&unix.IN_MOVE_SELF != 0 {
		parts = append(parts, "self movement")
	}
	if len(parts) == 0 {
		return "unknown"
	}

	return strings.Join(parts, ",")
}

// Get watching path by watch descriptor
func watchPath(watchPaths map[int]string, wd int) (string, error) {
	path, ok := watchPaths[wd]
	if !ok {
		return "", fmt.Errorf("no path for wd %d", wd)
	}
	return path, nil
}
