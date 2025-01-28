# Reverse Proxy

This project is a minimalist reverse proxy inspired by the architecture of Nginx. It leverages an event loop for handling multiple TCP network connections efficiently and follows a **master-slave architecture**. The goal of this project is to demonstrate a basic yet functional reverse proxy setup with inter-process communication using Unix sockets.

## Architecture Overview

### Key Features
- **Event Loop Model**: Efficiently handles multiple network connections by subscribing to events.
- **Master-Slave Architecture**: 
  - The master process is responsible for accepting new connections.
  - The master sends the connection's file descriptor to a child process (worker) for further handling.
- **Inter-Process Communication (IPC)**:
  - Utilizes Unix sockets for communication between the master and worker processes.
  - The master delegates tasks to workers, while the workers subscribe to the event loop for processing events.

### Typical Workflow
1. The **master process** initializes and begins listening for incoming connections.
2. Upon receiving a connection, the master passes the connection's file descriptor to a worker process using Unix sockets.
3. The **worker process** subscribes to the event loop and handles events (e.g., forwarding requests and responses).
4. The reverse proxy forwards requests to the appropriate backend server and sends responses back to the client.

## Prerequisites

- **Operating System**: Linux-based OS (e.g., Ubuntu, WSL) to utilize Unix sockets and event loop functionality. Note that a Linux or Ubuntu-based OS is required to test and run this application.
- **Go Version**: Go 1.20+

## Getting Started

### 1. Run the Master Process
Start the master process, which will also initialize and run worker processes:
```bash
go run master.go
```

### 2. Test the Reverse Proxy
To test the reverse proxy, you can run the sample backend server and send test requests:

#### Start the Backend Server
```bash
go run server.go
```

#### Test the Proxy
You can use tools like `curl` or a browser to send requests to the reverse proxy:
```bash
curl http://localhost:<proxy-port>
```
The proxy will forward the request to the backend server and return the response to the client.

## Notes

- **Minimal Features**: This is a basic implementation of a reverse proxy with limited functionality. Contributions to enhance features are welcome.
- **Limitations**:
  - No configuration support for load balancing or advanced routing.
  - Logging, error handling, and robustness improvements are needed.

## Contribution
If you'd like to contribute:
1. Fork the repository.
2. Create a feature branch (`git checkout -b feature-name`).
3. Commit your changes (`git commit -m "Add feature"`).
4. Push to the branch (`git push origin feature-name`).
5. Open a pull request.

## License
This project is licensed under the MIT License.

