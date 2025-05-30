package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	fmt.Println("Starting HTTP server on port 4221")

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err.Error())
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Println("Accepted connection from:", conn.RemoteAddr())

	reader := bufio.NewReader(conn)

	// Read the first line to get the request method, path, and HTTP version
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading request:", err.Error())
		return
	}

	// Parse the request line
	parts := strings.Split(strings.TrimSpace(requestLine), " ")
	if len(parts) != 3 {
		fmt.Println("Invalid HTTP request format")
		return
	}

	method := parts[0]
	path := parts[1]
	httpVersion := parts[2]

	fmt.Printf("Received %s request for %s using %s\n", method, path, httpVersion)

	// Read headers until we encounter an empty line
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading headers:", err.Error())
			return
		}

		// Empty line (just "\r\n") indicates the end of headers
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	// Send a simple 200 OK response
	response := "HTTP/1.1 200 OK\r\n\r\n"
	_, err = conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error sending response:", err.Error())
		return
	}

	fmt.Println("Response sent successfully")
}
