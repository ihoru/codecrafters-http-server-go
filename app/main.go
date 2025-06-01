package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
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
	requestHeaders := make(map[string]string)
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
			pair := strings.SplitN(line, ":", 2)
			if len(pair) == 2 {
				key := strings.ToLower(strings.TrimSpace(pair[0]))
				value := strings.TrimSpace(pair[1])
				requestHeaders[key] = value
			} else {
				fmt.Println("Invalid header format:", line)
			}
		}
	}

	parts := strings.Split(strings.TrimSpace(requestTarget), " ")
	if len(parts) != 3 {
		fmt.Println("Invalid HTTP request format")
		return
	}
	fmt.Println("Request:", requestTarget)

	method := parts[0]
	path := parts[1]
	httpVersion := parts[2]

	var statusLine string
	var body string
	headers := make(map[string]string)
	if httpVersion != "HTTP/1.1" {
		statusLine = "HTTP/1.1 426 Upgrade Required"
		headers["Upgrade"] = "HTTP/1.1"
	} else if method != "GET" {
		statusLine = "HTTP/1.1 405 Not Allowed"
	} else {
		statusLine = "HTTP/1.1 200 OK"
		if path == "/" {
		} else if path == "/user-agent" {
			body = requestHeaders["user-agent"]
		} else if after, found := strings.CutPrefix(path, "/echo/"); found {
			statusLine = "HTTP/1.1 200 OK"
			body = after
		} else {
			statusLine = "HTTP/1.1 404 Not Found"
		}
	}
	if body != "" {
		if headers["Content-Type"] == "" {
			headers["Content-Type"] = "text/plain"
		}
		headers["Content-Length"] = strconv.Itoa(len(body))
	}
	lines := make([]string, 0, 3+len(headers))
	lines = append(lines, statusLine)
	for k, v := range headers {
		lines = append(lines, fmt.Sprintf("%s: %s", k, v))
	}
	lines = append(lines, "")
	lines = append(lines, body)

	response := strings.Join(lines, "\r\n")
	_, err := conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error sending response:", err)
		return
	}

	fmt.Println("Response:", statusLine)
}
