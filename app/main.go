package main

import (
	"fmt"
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

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Println("Accepted connection from:", conn.RemoteAddr())

	//buf := make([]byte, 1024)
	//n, err := conn.Read(buf)
	//if err != nil {
	//	fmt.Println("Error reading request:", err)
	//	return
	//}

	response := "HTTP/1.1 200 OK\r\n\r\n"
	_, err := conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error sending response:", err)
		return
	}

	fmt.Println("Response sent successfully")
}
