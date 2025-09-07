package cli

import (
	"bufio"
	"cadence/constants"
	"cadence/server"
	"cadence/utils"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	// by default goes to local host port 6379
	port := flag.String("port", constants.DefaultPort, "the port to connect to")
	host := flag.String("host", "localhost", "the host to connect to")
	flag.Parse()

	fmt.Printf("Initiating connection to Cadence instance at %s:%s...\n", *host, *port)

	// connect to server
	conn, err := net.Dial("tcp", *host+":"+*port)
	defer conn.Close()
	if err != nil {
		fmt.Println("Could not connect to instance, exiting process...")
		return
	}
	dataChannel := utils.ReadFromConn(conn, func(r []string) []string { return r })

	fmt.Println("Connected successfully! Enter commands:")

	// start reading commands
	reader := bufio.NewReader(os.Stdin)
	for {
		// read one string at a time
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Retry, an error occurred client-side...")
			continue
		}
		parts := strings.Split(strings.TrimSpace(input), " ")

		if len(parts) != 0 {
			// a) print help message
			if strings.ToUpper(parts[0]) == "HELP" {
				fmt.Println("The valid commands and their use cases are as follows: ")
				fmt.Printf("%s - quickly check status of server (if alive should respond with %s)\n", server.Commands.STATUS, server.Responses.ALL_GOOD)
				fmt.Printf("%s args - returns back args\n", server.Commands.ECHO)
				fmt.Printf("%s - get info about DB instance\n", server.Commands.INFO)
				fmt.Printf("%s key - get value (if not set, returns nil string)\n", server.Commands.GET)
				fmt.Printf("%s key value [PX millis] - set value (optionally specify millis expiration)\n", server.Commands.GET)
				//TODO: add delete
				continue
			}

			// b) send data to server
			// NOTE: we don't perform client side validation because its unnecessary
			_, err = conn.Write(utils.BulkStringArraySerialize(parts))
			if err != nil {
				fmt.Printf("ERROR: connection to server has closed, please try connecting again.")
				break
			}

			// wait for and read response - EXPECT RESPONSE TO BE AN ARRAY OF LENGTH 1 WITH MESSAGE
			response, channelAlive := <-dataChannel
			if !channelAlive {
				fmt.Println("Connection closed, exiting process...")
				break
			}
			if len(response) == 1 {
				fmt.Println(response[0])
			} else {
				fmt.Println("ERROR: invalid response recieved, try again.")
			}
		}
	}
}
