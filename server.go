package main

import (
	"fmt"
	"net"
	"os"
)

// Ensures gofmt doesn't remove the "net" and "os" imports in stage 1 (feel free to remove this!)
var _ = net.Listen
var _ = os.Exit

func main() {
	fmt.Println("Logs from your program will appear here!")
	
	// bind to port 6379 to listen for incoming TCP connections
	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}

	// close binding after function exits
	defer l.Close()

	// wait for an incoming TCP connection
	c, err := l.Accept()
	if err != nil {
		fmt.Println("Error accepting connection: ", err.Error())
		os.Exit(1)
	}
	handleConnection(c)
}


func handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Println("Client connected:", conn.RemoteAddr())

	// create a buffer of bytes to read the input into
	buffer := make([]byte, 128)
	_,err := conn.Read(buffer)

	if err != nil {
		fmt.Println("Error reading from connection:", conn.RemoteAddr())
		return
	}

	fmt.Printf("Received: %s", buffer)
	conn.Write([]byte("+PONG\r\n"))

}
