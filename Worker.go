package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

const maxEvents = 1024

func main() {
	if len(os.Args) < 2 {
		fmt.Println("No worker ID provided")
		return
	}

	workerID := os.Args[1]
	fmt.Printf("Worker spawned with ID: %s\n", workerID)
	socketPath := fmt.Sprintf("/tmp/worker_%s.sock", workerID)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Println("Error using file descriptor:", err)
		return
	}
	defer conn.Close()

	unixConn := conn.(*net.UnixConn)
	file, err := unixConn.File()
	if err != nil {
		fmt.Println("Error getting file descriptor:", err)
		return
	}
	defer file.Close()

	fmt.Printf("File descriptor: %d\n", file.Fd())

	fmt.Println("Connected to parent process")

	masterFD := file.Fd()

	// Create an epoll instance
	epfd, err := unix.EpollCreate1(0)
	if err != nil {
		slog.Error("failed to create epoll instance")
		os.Exit(1)
	}
	defer unix.Close(epfd)

	event := &unix.EpollEvent{Events: unix.EPOLLIN, Fd: int32(masterFD)}
	if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, int(masterFD), event); err != nil {
		slog.Error("failed to add the file descriptor")
		os.Exit(1)
	}

	events := make([]unix.EpollEvent, maxEvents)

	for {
		n, err := unix.EpollWait(epfd, events, -1)
		if err != nil {
			slog.Error("failed to wait for epoll events", err)
			continue
		}

		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)

			if fd == int(masterFD) {

				oob := make([]byte, unix.CmsgSpace(4))
				buf := make([]byte, 1024)

				_, oobn, _, _, err := unix.Recvmsg(fd, buf, oob, 0)
				if err != nil {
					slog.Error("failed to receive message", err)
					return
				}

				cmsgs, err := unix.ParseSocketControlMessage(oob[:oobn])
				if err != nil {
					slog.Error("failed to parse socket control message", err)
					return
				}

				for _, cmsg := range cmsgs {
					if cmsg.Header.Level == unix.SOL_SOCKET && cmsg.Header.Type == unix.SCM_RIGHTS {
						fds, err := unix.ParseUnixRights(&cmsg)
						if err != nil {
							slog.Error("failed to parse Unix rights", err)
							return
						}

						// Use the received file descriptor
						receivedFD := fds[0]
						slog.Info("Received file descriptor", "fd", receivedFD)

						// Send "Hello, world!" to the TCP connection
						message := fmt.Sprintf("Hello, world! \n and req handeld by worker %d \n", workerID)
						_, err = unix.Write(receivedFD, []byte(message))
						if err != nil {
							slog.Error("error writing to received file descriptor", err)
							return
						}

						fmt.Printf("Sent message to TCP connection: %s\n", message)
					}
				}

			}
		}
	}
}
