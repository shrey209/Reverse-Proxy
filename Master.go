package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

type ReverseProxy struct {
	Port           string
	NumWorkers     int
	RequestTimeout int
	connections    []*net.Conn
	index          int
}

func (ReverseProxy *ReverseProxy) spawnWorker(workerID int) {
	socketPath := fmt.Sprintf("/tmp/worker_%d.sock", workerID)

	// Remove any existing socket file
	err := os.Remove(socketPath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("Failed to remove existing socket: %v", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to create Unix socket: %v", err)
	}

	cmd := exec.Command("go", "run", "Worker.go", fmt.Sprintf("%d", workerID))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	if err != nil {
		log.Fatalf("Error starting worker process: %v", err)
	}

	conn, err := listener.Accept()
	if err != nil {
		log.Fatalf("Failed to accept connection: %v", err)
	}

	ReverseProxy.connections = append(ReverseProxy.connections, &conn)
	log.Println("Worker connected.")
}

func (ReverseProxy *ReverseProxy) CreateWorkers() {
	for i := 1; i <= ReverseProxy.NumWorkers; i++ {
		ReverseProxy.spawnWorker(i)
	}
	log.Print("All workers spawned successfully")
}

func main() {
	proxy := ReverseProxy{
		Port:           ":8080",
		RequestTimeout: 30,
		NumWorkers:     2,
	}

	proxy.CreateWorkers()

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Error starting TCP listener: %v", err)
	}
	defer listener.Close()

	fmt.Println("Listening on port 8080...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		tcpConn, ok := conn.(*net.TCPConn)
		if !ok {
			fmt.Println("Not a TCP connection")
			conn.Close()
			continue
		}

		file, err := tcpConn.File()
		if err != nil {
			fmt.Println("Error getting file descriptor:", err)
			conn.Close()
			continue
		}
		defer file.Close()

		fd := file.Fd()
		log.Printf("File descriptor fd of this TCP connection: %d", fd)

		proxy.sendFileDescriptor(fd)
	}
}

func (ReverseProxy *ReverseProxy) sendFileDescriptor(fd uintptr) {

	conn := ReverseProxy.connections[ReverseProxy.index]
	unixConn, ok := (*conn).(*net.UnixConn)
	if !ok {
		log.Println("Failed to cast to UnixConn")
		return
	}

	rights := unix.UnixRights(int(fd))
	_, _, err := unixConn.WriteMsgUnix(nil, rights, nil)
	if err != nil {
		log.Printf("Failed to send file descriptor: %v", err)
		return
	}

	ReverseProxy.index = (ReverseProxy.index + 1) % len(ReverseProxy.connections)
}
