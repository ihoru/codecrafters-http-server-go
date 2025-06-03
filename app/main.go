package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// HTTP status codes
const (
	StatusOK                  = "HTTP/1.1 200 OK"
	StatusCreated             = "HTTP/1.1 201 Created"
	StatusBadRequest          = "HTTP/1.1 400 Bad Request"
	StatusNotFound            = "HTTP/1.1 404 Not Found"
	StatusMethodNotAllowed    = "HTTP/1.1 405 Not Allowed"
	StatusConflict            = "HTTP/1.1 409 Conflict"
	StatusUpgradeRequired     = "HTTP/1.1 426 Upgrade Required"
	StatusInternalServerError = "HTTP/1.1 500 Internal Server Error"
)

// Request represents an HTTP request
type Request struct {
	Method      string
	Path        string
	HTTPVersion string
	Headers     map[string]string
	Body        []byte
}

// Response represents an HTTP response
type Response struct {
	StatusLine string
	Headers    map[string]string
	Body       string
}

// Handler is an interface for handling HTTP requests
type Handler interface {
	Handle(req *Request) *Response
}

// HandlerFunc is a function type that implements the Handler interface
type HandlerFunc func(req *Request) *Response

// Handle calls the handler function
func (f HandlerFunc) Handle(req *Request) *Response {
	return f(req)
}

// Middleware wraps a handler with additional functionality
type Middleware func(Handler) Handler

// Chain combines multiple middleware into a single middleware
func Chain(middlewares ...Middleware) Middleware {
	return func(handler Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}
		return handler
	}
}

func main() {
	directory := parseArgs()

	fmt.Println("Starting HTTP server on port 4221")
	if directory != "" {
		fmt.Println("Directory:", directory)
	}

	// Create middleware chain
	handler := createMiddlewareChain(directory)

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

		go handleConnection(conn, handler)
	}
}

// httpVersionMiddleware checks that the HTTP version is HTTP/1.1
func httpVersionMiddleware(next Handler) Handler {
	return HandlerFunc(func(req *Request) *Response {
		if req.HTTPVersion != "HTTP/1.1" {
			return &Response{
				StatusLine: StatusUpgradeRequired,
				Headers: map[string]string{
					"Upgrade": "HTTP/1.1",
				},
			}
		}
		return next.Handle(req)
	})
}

// methodValidationMiddleware validates that the HTTP method is GET or POST
func methodValidationMiddleware(next Handler) Handler {
	return HandlerFunc(func(req *Request) *Response {
		if req.Method != "GET" && req.Method != "POST" {
			return &Response{
				StatusLine: StatusMethodNotAllowed,
				Headers:    make(map[string]string),
			}
		}
		return next.Handle(req)
	})
}

// compressionMiddleware adds Content-Encoding: gzip header and compresses the response body if client supports it
func compressionMiddleware(next Handler) Handler {
	return HandlerFunc(func(req *Request) *Response {
		response := next.Handle(req)

		// Check if client supports gzip compression
		acceptEncoding, ok := req.Headers["accept-encoding"]
		if ok && response.Body != "" {
			// Split by comma and check each encoding
			encodings := strings.Split(acceptEncoding, ",")
			for _, encoding := range encodings {
				// Trim whitespace and convert to lowercase
				encoding = strings.TrimSpace(strings.ToLower(encoding))
				if encoding == "gzip" {
					if response.Headers == nil {
						response.Headers = make(map[string]string)
					}

					// Compress the response body using gzip
					var compressedBody bytes.Buffer
					gz := gzip.NewWriter(&compressedBody)
					if _, err := gz.Write([]byte(response.Body)); err != nil {
						fmt.Println("Error compressing response body:", err)
						return response
					}
					if err := gz.Close(); err != nil {
						fmt.Println("Error closing gzip writer:", err)
						return response
					}

					// Update the response with compressed body
					response.Body = string(compressedBody.Bytes())
					response.Headers["Content-Encoding"] = "gzip"

					// Update Content-Length header
					response.Headers["Content-Length"] = strconv.Itoa(len(response.Body))
					break
				}
			}
		}

		return response
	})
}

// routingMiddleware routes requests to appropriate handlers
func routingMiddleware(directory string) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(req *Request) *Response {
			// Route to appropriate handler
			switch {
			case req.Method == "GET" && req.Path == "/":
				// Root path, just return 200 OK
				return &Response{
					StatusLine: StatusOK,
					Headers:    make(map[string]string),
				}

			case req.Method == "GET" && req.Path == "/user-agent":
				return handleUserAgent(req)

			case req.Method == "GET" && strings.HasPrefix(req.Path, "/echo/"):
				return handleEcho(req)

			case strings.HasPrefix(req.Path, "/files/"):
				return handleFiles(req, directory)

			default:
				return next.Handle(req)
			}
		})
	}
}

// createMiddlewareChain creates the middleware chain for request handling
func createMiddlewareChain(directory string) Handler {
	// Create base handler that returns 404 Not Found
	notFoundHandler := HandlerFunc(func(req *Request) *Response {
		return &Response{
			StatusLine: StatusNotFound,
			Headers:    make(map[string]string),
		}
	})

	// Build middleware chain
	middlewareChain := Chain(
		httpVersionMiddleware,
		methodValidationMiddleware,
		compressionMiddleware,
		routingMiddleware(directory),
	)

	// Apply middleware chain to base handler
	return middlewareChain(notFoundHandler)
}

// parseArgs parses command line arguments and returns the directory if specified
func parseArgs() string {
	var directory string

	// Check for --directory flag
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--directory" && i+1 < len(os.Args) {
			directory = os.Args[i+1]
			i++ // Skip the next argument as we've already processed it
		}
	}

	return directory
}

// handleConnection handles a client connection
func handleConnection(conn net.Conn, handler Handler) {
	defer conn.Close()

	fmt.Println("Accepted connection from:", conn.RemoteAddr())

	request, err := parseRequest(conn)
	if err != nil {
		fmt.Println("Error parsing request:", err)
		return
	}

	fmt.Println("Request:", request.Method, request.Path, request.HTTPVersion)

	response := handler.Handle(request)

	err = sendResponse(conn, response)
	if err != nil {
		fmt.Println("Error sending response:", err)
		return
	}

	fmt.Println("Response:", response.StatusLine)
}

// parseRequest parses an HTTP request from a connection
func parseRequest(conn net.Conn) (*Request, error) {
	reader := bufio.NewReader(conn)
	requestHeaders := make(map[string]string)
	var requestTarget string
	var requestBody []byte

	// Read until we get the empty line that marks end of headers
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed by client")
		}
		if err != nil {
			return nil, fmt.Errorf("error reading: %w", err)
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
			return nil, fmt.Errorf("error reading request body: %w", err)
		}
	}

	parts := strings.Split(strings.TrimSpace(requestTarget), " ")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid HTTP request format")
	}

	return &Request{
		Method:      parts[0],
		Path:        parts[1],
		HTTPVersion: parts[2],
		Headers:     requestHeaders,
		Body:        requestBody,
	}, nil
}

// handleUserAgent handles the /user-agent endpoint
func handleUserAgent(req *Request) *Response {
	return &Response{
		StatusLine: StatusOK,
		Headers:    make(map[string]string),
		Body:       req.Headers["user-agent"],
	}
}

// handleEcho handles the /echo/ endpoint
func handleEcho(req *Request) *Response {
	content := strings.TrimPrefix(req.Path, "/echo/")
	return &Response{
		StatusLine: StatusOK,
		Headers:    make(map[string]string),
		Body:       content,
	}
}

// handleFiles handles the /files/ endpoint for both GET and POST methods
func handleFiles(req *Request, directory string) *Response {
	response := &Response{
		StatusLine: StatusOK,
		Headers:    make(map[string]string),
	}
	if directory == "" {
		response.StatusLine = StatusBadRequest
		fmt.Println("Directory not specified for /files endpoint")
		return response
	}

	filePath := filepath.Clean(strings.TrimPrefix(req.Path, "/files/"))
	if filePath == "" {
		response.StatusLine = StatusBadRequest
		fmt.Println("Invalid file path:", filePath)
		return response
	}
	// Check if path attempts to traverse up
	if strings.Contains(filePath, "..") {
		// Prevent directory traversal attacks
		response.StatusLine = StatusBadRequest
		fmt.Println("Invalid file path (directory traversal):", filePath)
		return response
	}

	fullPath := filepath.Join(directory, filePath)

	if req.Method == "POST" {
		return handleFileUpload(req, fullPath)
	} else if req.Method == "GET" {
		return handleFileDownload(req, fullPath)
	} else {
		response.StatusLine = StatusMethodNotAllowed
		return response
	}
}

// handleFileUpload handles uploading a file (POST to /files/)
func handleFileUpload(req *Request, fullPath string) *Response {
	response := &Response{
		StatusLine: StatusOK,
		Headers:    make(map[string]string),
	}

	if req.Body == nil {
		response.StatusLine = StatusBadRequest
		fmt.Println("No request body provided for POST method")
		return response
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		response.StatusLine = StatusInternalServerError
		fmt.Println("Error creating directory:", err)
		return response
	}

	// Check if the file already exists
	if _, err := os.Stat(fullPath); err == nil {
		response.StatusLine = StatusConflict
		fmt.Println("File already exists:", fullPath)
		return response
	} else if !os.IsNotExist(err) {
		response.StatusLine = StatusInternalServerError
		fmt.Println("Error checking file existence:", err)
		return response
	}

	// Create a new file with the content from the request body
	if err := os.WriteFile(fullPath, req.Body, 0644); err != nil {
		response.StatusLine = StatusInternalServerError
		fmt.Println("Error creating file:", err)
		return response
	}

	response.StatusLine = StatusCreated
	return response
}

// handleFileDownload handles downloading a file (GET from /files/)
//
//goland:noinspection GoUnusedParameter
func handleFileDownload(req *Request, fullPath string) *Response {
	response := &Response{
		StatusLine: StatusOK,
		Headers:    make(map[string]string),
	}

	fileInfo, err := os.Stat(fullPath)
	if err != nil || fileInfo.IsDir() {
		response.StatusLine = StatusNotFound
		return response
	}

	// Read the file content
	file, err := os.Open(fullPath)
	if err != nil {
		response.StatusLine = StatusInternalServerError
		fmt.Println("Error opening file:", err)
		return response
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		response.StatusLine = StatusInternalServerError
		fmt.Println("Error reading file:", err)
		return response
	}

	response.Body = string(fileContent)
	response.Headers["Content-Type"] = "application/octet-stream"
	response.Headers["Content-Disposition"] = fmt.Sprintf("attachment; filename=%s", filepath.Base(fullPath))

	return response
}

// sendResponse sends an HTTP response to the client
func sendResponse(conn net.Conn, response *Response) error {
	// Add Content-Length and Content-Type headers if body is not empty
	if response.Body != "" {
		if response.Headers["Content-Type"] == "" {
			response.Headers["Content-Type"] = "text/plain"
		}
		response.Headers["Content-Length"] = strconv.Itoa(len(response.Body))
	}

	// Build response
	lines := make([]string, 0, 3+len(response.Headers))
	lines = append(lines, response.StatusLine)
	for k, v := range response.Headers {
		lines = append(lines, fmt.Sprintf("%s: %s", k, v))
	}
	lines = append(lines, "")
	lines = append(lines, response.Body)

	responseStr := strings.Join(lines, "\r\n")
	_, err := conn.Write([]byte(responseStr))
	return err
}
