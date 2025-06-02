package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	var directory string

	// Check for --directory flag
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--directory" && i+1 < len(os.Args) {
			directory = os.Args[i+1]
			i++ // Skip the next argument as we've already processed it
		}
	}

	fmt.Println("Starting HTTP server on port 4221")
	if directory != "" {
		fmt.Println("Directory:", directory)
	}

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

		go handleConnection(conn, directory)
	}
}

func handleConnection(conn net.Conn, directory string) {
	defer conn.Close()

	fmt.Println("Accepted connection from:", conn.RemoteAddr())

	var requestTarget string
	reader := bufio.NewReader(conn)
	requestHeaders := make(map[string]string)
	var requestBody []byte

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

	// Read request body if Content-Length header is present
	if contentLength, err := strconv.Atoi(requestHeaders["content-length"]); err == nil && contentLength > 0 {
		requestBody = make([]byte, contentLength)
		_, err = io.ReadFull(reader, requestBody)
		if err != nil {
			fmt.Println("Error reading request body:", err)
			return
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
	} else if method != "GET" && method != "POST" {
		statusLine = "HTTP/1.1 405 Not Allowed"
	} else {
		statusLine = "HTTP/1.1 200 OK"
		if method == "GET" && path == "/" {
			// pass
		} else if method == "GET" && path == "/user-agent" {
			body = requestHeaders["user-agent"]
		} else if after, found := strings.CutPrefix(path, "/echo/"); found && method == "GET" {
			statusLine = "HTTP/1.1 200 OK"
			body = after
		} else if directory != "" && strings.HasPrefix(path, "/files/") {
			filePath := filepath.Clean(strings.TrimPrefix(path, "/files/"))
			if filePath == "" {
				statusLine = "HTTP/1.1 400 Bad Request"
				fmt.Println("Invalid file path:", filePath)
			} else {
				fullPath := filepath.Join(directory, filePath)
				if method == "POST" {
					if requestBody == nil {
						statusLine = "HTTP/1.1 400 Bad Request"
						fmt.Println("No request body provided for POST method")
					} else if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
						// Ensure the directory exists
						statusLine = "HTTP/1.1 500 Internal Server Error"
						fmt.Println("Error creating directory:", err)
					} else if _, err := os.Stat(fullPath); err == nil {
						// Check if the file already exists
						statusLine = "HTTP/1.1 409 Conflict"
						fmt.Println("File already exists:", fullPath)
					} else if !os.IsNotExist(err) {
						statusLine = "HTTP/1.1 500 Internal Server Error"
						fmt.Println("Error checking file existence:", err)
					} else {
						// Create a new file with the content from the request body
						err := os.WriteFile(fullPath, requestBody, 0644)
						if err != nil {
							statusLine = "HTTP/1.1 500 Internal Server Error"
							fmt.Println("Error creating file:", err)
						} else {
							statusLine = "HTTP/1.1 201 Created"
						}
					}
				} else { // GET method
					fileInfo, err := os.Stat(fullPath)
					if err != nil || fileInfo.IsDir() {
						statusLine = "HTTP/1.1 404 Not Found"
					} else {
						// Read the file content
						file, err := os.Open(fullPath)
						if err != nil {
							statusLine = "HTTP/1.1 500 Internal Server Error"
							fmt.Println("Error opening file:", err)
						} else {
							defer file.Close()
							fileContent, err := io.ReadAll(file)
							if err != nil {
								statusLine = "HTTP/1.1 500 Internal Server Error"
								fmt.Println("Error reading file:", err)
							} else {
								body = string(fileContent)
								headers["Content-Type"] = "application/octet-stream"
								headers["Content-Disposition"] = fmt.Sprintf("attachment; filename=%s", filepath.Base(fullPath))
								headers["Content-Length"] = strconv.Itoa(len(fileContent))
							}
						}
					}
				}
			}
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
