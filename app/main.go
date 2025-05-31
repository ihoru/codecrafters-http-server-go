package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
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
		fmt.Println("Received header:", line)
	}

	response := "HTTP/1.1 200 OK\r\n\r\n"
	_, err := conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error sending response:", err)
		return
	}

	fmt.Println("Response sent successfully")
}
