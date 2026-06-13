package util

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
func ListenInotifyEvents(watchPaths map[int]string, ctx context.Context, fanFd int) error {
	// 1.Listen indefinitely
	buf := make([]byte, 4096)
	pfds := []unix.PollFd{{Fd: int32(fanFd), Events: unix.POLLIN}}

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
		n, err := unix.Read(fanFd, buf)
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

		// 4.Read binary buffer
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

		var name string
		if event.Len > 0 {
			nameBytes := make([]byte, event.Len)
			if _, err := io.ReadFull(bufReader, nameBytes); err != nil {
				fmt.Printf("Problem while reading inotify event name: %v\n", err)
				return err
			}
			name = strings.TrimRight(string(nameBytes), "\x00")
		}

		eventPath, err := getWatchingPath(watchPaths, int(event.Wd))
		if err != nil {
			fmt.Println(err)
		} else {
			eventPathInode := getPathInode(eventPath)

			if name != "" {
				childPath := filepath.Join(*eventPath, name)
				childPathInode := getPathInode(&childPath)
				fmt.Printf("Got event: wd=%d mask=%s path=%s name=%s inodeParent=%d inodeChild=%d\n", event.Wd, getEventStrRepresentation(event.Mask), *eventPath, name, eventPathInode, childPathInode)

				if eventMask&unix.IN_CREATE != 0 || eventMask&unix.IN_MOVED_TO != 0 {
					childWd, err := unix.InotifyAddWatch(inFd, childPath, uint32(eventsMask))
					if err != nil {
						fmt.Printf("Error while adding %s to the inotify group: %v\n", childPath, err)
						continue
					}
					watchPaths[childWd] = childPath
				} else if eventMask&unix.IN_DELETE != 0 || eventMask&unix.IN_MOVED_FROM != 0 {
					delete(watchPaths, childWd)
				}
			} else {
				fmt.Printf("Got event: wd=%d mask=%s path=%s inode=%d\n", event.Wd, getEventStrRepresentation(event.Mask), *eventPath, eventPathInode)
			}
		}
	}
	return nil
}

// Get event mask in string representation
func getEventStrRepresentation(eventMask uint32) string {
	representation := make([]string, 0)

	if eventMask&unix.IN_MODIFY != 0 {
		representation = append(representation, "modification")
	}

	if eventMask&unix.IN_ATTRIB != 0 {
		representation = append(representation, "change attributes")
	}

	if eventMask&unix.IN_DELETE_SELF != 0 {
		representation = append(representation, "self deletion")
	}

	if eventMask&unix.IN_MOVE_SELF != 0 {
		representation = append(representation, "self movement")
	}

	if eventMask&unix.IN_CREATE != 0 {
		representation = append(representation, "inner item creation")
	}

	if eventMask&unix.IN_DELETE != 0 {
		representation = append(representation, "inner item deletion")
	}

	if eventMask&unix.IN_MOVED_FROM != 0 {
		representation = append(representation, "inner item move from")
	}

	if eventMask&unix.IN_MOVED_TO != 0 {
		representation = append(representation, "inner item move to")
	}

	return strings.Join(representation, ",")
}

// Get watching path by watch descriptor
func getWatchingPath(watchPaths map[int]string, watchDescriptor int) (*string, error) {
	path, in := watchPaths[watchDescriptor]
	if !in {
		return nil, fmt.Errorf("no path with wd %d", watchDescriptor)
	}

	return &path, nil
}

// Get inode of the path
func getPathInode(path *string) uint64 {
	stat, err := os.Stat(*path)
	if err != nil {
		return 0
	}
	return stat.Sys().(*syscall.Stat_t).Ino
}
