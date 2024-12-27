package main

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os/exec"
	"syscall"
)

type ReverseProxy struct {
	Port            string     // Port on which the proxy server listens.
	UpstreamServers []*url.URL // List of upstream server URLs.
	NumWorkers      int        // Number of workers to handle requests.
	RequestTimeout  int        // Timeout for requests in seconds.
	UDSListeners    []net.Listener
}

func (ReverseProxy *ReverseProxy) spawnWorker(workerID int) {

	udsPath := fmt.Sprintf("/tmp/reverseproxy_worker_%d.sock", workerID)
	listener, err := net.Listen("unix", udsPath)
	if err != nil {
		log.Fatalf("Error creating UDS for worker %d: %v", workerID, err)
	}
	defer listener.Close()

	// Set the listener to non-blocking mode
	file, err := listener.(*net.UnixListener).File()
	if err != nil {
		log.Fatalf("Error getting file from listener: %v", err)
	}
	defer file.Close()

	// Make the UDS socket non-blocking
	if err := syscall.SetNonblock(int(file.Fd()), true); err != nil {
		log.Fatalf("Error setting UDS socket to non-blocking: %v", err)
	}

	// Add the listener to the UDSListeners slice
	ReverseProxy.UDSListeners = append(ReverseProxy.UDSListeners, listener)

	// Start the worker process and pass the UDS path as an argument
	cmd := exec.Command("go", "run", "Worker.go", fmt.Sprintf("%d", workerID), udsPath)
	err = cmd.Start()
	if err != nil {
		log.Printf("Error starting worker %d: %v\n", workerID, err)
		return
	}

	// Log worker creation
	log.Printf("Worker %d started and listening on UDS %s\n", workerID, udsPath)
}

func (ReverseProxy *ReverseProxy) CreateWorkers() {
	for i := 1; i <= ReverseProxy.NumWorkers; i++ {
		ReverseProxy.spawnWorker(i) // Call spawnWorkers with a unique workerID
	}
	log.Print("all workers spawned succesfully")
}

func mustParseURL(rawURL string) *url.URL {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		log.Fatalf("Failed to parse URL: %v", err)
	}
	return parsedURL
}

func main() {
	// Define a reverse proxy instance
	proxy := ReverseProxy{
		Port:            ":8080",
		UpstreamServers: []*url.URL{mustParseURL("http://localhost:9000")},
		RequestTimeout:  30,
		NumWorkers:      5,
	}
	listener, err := net.Listen("tcp", proxy.Port)
	if err != nil {
		log.Fatalf("Error starting TCP listener: %v", err)
	}
	defer listener.Close()

	fmt.Println("Listening on port 8080...")

	con := 0

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		uc, ok := conn.(*net.UnixConn)
		if !ok {
			log.Printf("Failed to convert connection to UnixConn")
			continue
		}

		file, err := uc.File()
		if err != nil {
			log.Printf("Error getting file descriptor: %v", err)
			continue
		}
		defer file.Close()

		fd := file.Fd()

		err = sendFileDescriptor(fd, fmt.Sprintf("/tmp/reverseproxy_worker_%d.sock", con%proxy.NumWorkers))
		if err != nil {
			log.Printf("Error sending FD to worker: %v", err)
		}
	}

}

func sendFileDescriptor(fd uintptr, udsPath string) error {
	// Connect to the worker's UDS
	conn, err := net.Dial("unix", udsPath)
	if err != nil {
		return fmt.Errorf("failed to connect to worker's UDS: %v", err)
	}
	defer conn.Close()

	// Prepare the message with the file descriptor
	controlMessage := []byte(fmt.Sprintf("FD %v", fd))

	// Send the control message
	_, err = conn.Write(controlMessage)
	if err != nil {
		return fmt.Errorf("failed to send file descriptor: %v", err)
	}

	return nil
}
