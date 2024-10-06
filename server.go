package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"cadence/commands"
	"cadence/parser"
)

func main() {
	fmt.Println("Starting server...")
	
	// bind to port 6379 to listen for incoming TCP connections
	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}

	// close binding after function exits
	defer l.Close()

	// wait for an incoming TCP connection
	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConnection(c)
	}
}


func handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Println("Client connected:", conn.RemoteAddr())

	// create a buffer of bytes to read the input into
	buffer := make([]byte, 1024)
	for {
		// read from connection
		_,err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Closing connection...")
			} else {
				fmt.Println("Error reading from connection:", conn.RemoteAddr())
			}
			return
		}

		// get deserialized commands + args
		parts := parser.RESPDeserialize(string(buffer))
		fmt.Println(parts)

		// decide action to take based on first (should be command)
		if(len(parts) == 0) {
			fmt.Println("No commands passed - nothing to do")
		} else {
			command := strings.ToLower(parts[0])
			cmdFunc, exists := commands.CmdMap[command]
			if exists {
				fmt.Println("Executing command:", command)
				cmdFunc(conn, parts[1:])
			} else {
				fmt.Println("Unknown command passed:", command)
			}
		}
	}
}