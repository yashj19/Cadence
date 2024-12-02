package main

import (
	"bufio"
	"cadence/commands"
	"cadence/parser"
	readutils "cadence/read-utils"
	"cadence/shared"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	// by default goes to local host port 6379
	port := flag.String("port", shared.DefaultPort, "the port to connect to")
	host := flag.String("host", "localhost", "the host to connect to")
	flag.Parse()

	fmt.Printf("Initiating connection to Cadence instance at %s:%s...\n", *host, *port)

	conn, err := net.Dial("tcp", *host+":"+*port)

	if err != nil {
		fmt.Println("Could not connect to instance, exiting process...")
		return
	}
	instChannel := readutils.ReadFromConn(conn)

	fmt.Println("Connected successfully! Enter commands:")

	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Retry, an error occurred client-side...")
			continue
		}

		parts := strings.Split(strings.TrimSpace(input), " ")
		if len(parts) != 0 {
			if strings.ToUpper(parts[0]) == "HELP" {
				fmt.Println("The valid commands and their use cases are as follows: ")
				fmt.Printf("%s - quickly check status of server (if alive should respond with %s)\n", commands.Commands.STATUS, commands.Responses.ALL_GOOD)
				fmt.Printf("%s args - returns back args\n", commands.Commands.ECHO)
				fmt.Printf("%s - get info about DB instance\n", commands.Commands.INFO)
				fmt.Printf("%s key - get value (if not set, returns nil string)\n", commands.Commands.GET)
				fmt.Printf("%s key value [PX millis] - set value (optionally specify millis expiration)\n", commands.Commands.GET)
				//TODO: add delete
				continue
			}
			instruction := commands.NewInstruction(parts)
			valid, errReason := instruction.Validate()
			if valid {
				// fmt.Println("Instruction is valid, sending to DB.")
				// write the instruction
				_, err = conn.Write(parser.BulkStringArraySerialize(parts))
				if err != nil {
					fmt.Printf("ERROR: connection to server has closed, please try again.")
				}

				// wait for and read response
				response, channelAlive := <-instChannel
				if !channelAlive {
					fmt.Println("Connection closed, exiting process...")
					// TODO: make sure happens even if not awaiting a response from db
					break
				}
				response.Print()
			} else {
				fmt.Printf("ERROR: %s Type 'HELP' to see a list of all valid commands and their use cases.\n", errReason)
			}
		}
	}
}
