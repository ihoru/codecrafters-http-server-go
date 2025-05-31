package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

func main() {
	fmt.Println("Starting HTTP server on port 4221")

	listener, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221", err)
		os.Exit(1)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Println("Accepted connection from:", conn.RemoteAddr())

	var requestTarget string
	reader := bufio.NewReader(conn)
	// Read until we get the empty line that marks end of headers
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			fmt.Println("Connection closed by client")
			return
		}
		if err != nil {
			fmt.Println("Error reading:", err)
			return
		}
		if line == "\r\n" || line == "\n" { // End of headers
			break
		}
		line = line[:len(line)-1] // Remove trailing newline
		if requestTarget == "" {
			requestTarget = line
		} else {
			//fmt.Println("Received header:", line)
		}
	}

	parts := strings.Split(strings.TrimSpace(requestTarget), " ")
	if len(parts) != 3 {
		fmt.Println("Invalid HTTP request format")
		return
	}
	fmt.Println("requestTarget:", requestTarget)

	method := parts[0]
	path := parts[1]
	httpVersion := parts[2]

	var response string
	if method == "GET" && path == "/" && httpVersion == "HTTP/1.1" {
		response = "HTTP/1.1 200 OK"
	} else {
		response = "HTTP/1.1 404 Not Found"
	}
	_, err := conn.Write([]byte(response + "\r\n\r\n"))
	if err != nil {
		fmt.Println("Error sending response:", err)
		return
	}

	fmt.Println("Response sent successfully:", response)
}
