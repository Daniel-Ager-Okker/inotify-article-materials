package util

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

// Fixed-size part of C-kernel `inotify_event` (name[] is variable-length, read separately).
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
			return fmt.Errorf("poll(): %w", err)
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
			return fmt.Errorf("read(): %w", err)
		}
		if n <= 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// 5.Read binary buffer
		err = readInotifyBuffer(watchPaths, buf, n)
		if err != nil {
			return err
		}
	}

	return ctx.Err()
}

// Parse inotify binary buffer
func readInotifyBuffer(watchPaths map[int]string, buf []byte, bufSize int) error {
	bufReader := bytes.NewReader(buf[:bufSize])

	for bufReader.Len() > 0 {
		var event inotifyEvent

		err := binary.Read(bufReader, binary.LittleEndian, &event)
		if err != nil {
			fmt.Printf("Problem while reading inotify event buffer: %v\n", err)
			return err
		}

		eventPath, err := getWatchingPath(watchPaths, int(event.Wd))
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Got event: wd=%d mask=%s path=%s\n", event.Wd, getEventStrRepresentation(event.Mask), *eventPath)
		}
	}
	return nil
}

// Get event mask in string representation
func getEventStrRepresentation(eventMask uint32) string {
	if eventMask&unix.IN_MODIFY != 0 {
		return "modification"
	}

	if eventMask&unix.IN_ATTRIB != 0 {
		return "change attributes"
	}

	if eventMask&unix.IN_DELETE_SELF != 0 {
		return "self deletion"
	}

	if eventMask&unix.IN_MOVE_SELF != 0 {
		return "self movement"
	}

	return "N/A"
}

// Get watching path by watch descriptor
func getWatchingPath(watchPaths map[int]string, watchDescriptor int) (*string, error) {
	path, in := watchPaths[watchDescriptor]
	if !in {
		return nil, fmt.Errorf("no path with wd %d", watchDescriptor)
	}

	return &path, nil
}
