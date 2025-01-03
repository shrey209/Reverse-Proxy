package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"

	"golang.org/x/sys/unix"
)

const maxEvents = 1024

var proxymap = make(map[int]int)
var upstreamServer = "localhost:8080"

type proxy struct {
}

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

						//subscribe to epoll
						connEvent := &unix.EpollEvent{Events: unix.EPOLLIN, Fd: int32(receivedFD)}
						if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, int(receivedFD), connEvent); err != nil {
							slog.Error("failed to add connection to epoll", err)
							conn.Close()
						}
						slog.Info("new connection accepted", "address", conn.RemoteAddr().String())
					}
				}

			} else {
				log.Println(" the http proxy fd is :- %d", fd)
				buffer := make([]byte, 1024)
				n, err := unix.Read(fd, buffer)
				if err != nil {
					slog.Error("some error occured")
				}
				str := string(buffer[:n])

				reader := bytes.NewReader([]byte(str))
				req, err := http.ReadRequest(bufio.NewReader(reader))
				if err != nil {
					fmt.Println("Error parsing request:", err)
					return
				}
				req.URL.Scheme = "http"
				req.URL.Host = "localhost:8000"
				req.RequestURI = ""
				req.Header.Del("Connection")

				conn, err = net.Dial("tcp", "localhost:8000")
				if err != nil {
					log.Fatal("Error connecting to upstream server:", err)
				}

				err = req.Write(conn)
				if err != nil {
					log.Fatalf("Failed to write request to upstream server: %v", err)
				}

				response, err := http.ReadResponse(bufio.NewReader(conn), req)
				if err != nil {
					log.Fatalf("Failed to read response from upstream server: %v", err)
				}

				var responseBuffer bytes.Buffer
				err = response.Write(&responseBuffer)
				if err != nil {
					log.Fatalf("Failed to serialize response: %v", err)
				}
				_, err = unix.Write(fd, responseBuffer.Bytes())
				if err != nil {
					println("error occured on writing ")
				}
				unix.Close(fd)
			}
		}
	}
}
