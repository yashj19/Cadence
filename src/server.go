package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/pkg/errors"
)

var port *string
var masterHost = "";
var masterPort = "";
var replID string = "8371b4fb1155b71f4a04d3e1bc3e18c4a990aeeb";
var repOffset int = 0;
var myRepOffset int = 0;

func main() {

	// get and parse flag for which port it is
	port = flag.String("port", "6379", "the port at which to run the db")
	replicaOf := flag.String("replicaof", "none", "the host and port that it is a replica of")
	flag.Parse()

	fmt.Println("Starting server at port " + *port + "...")
	
	// bind to port 6379 to listen for incoming TCP connections
	l, err := net.Listen("tcp", ":" + *port)
	if err != nil {
		fmt.Println("Failed to bind to port " + *port)
		os.Exit(1)
	}

	// close binding after function exits
	defer l.Close()

	// if its a replica, begin handshake with master
	if *replicaOf != "none" {
		masterConn, err := handshakeMaster(*replicaOf)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		go handleMasterConnection(masterConn)
	}

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

func handshakeMaster(replicaOfInfo string) (net.Conn, error) {
	parts := strings.Split(replicaOfInfo, " ")
	masterHost = parts[0]
	masterPort = parts[1]

	// first, create tcp connection with master and send "PING"
	conn, err := net.Dial("tcp", masterHost + ":" + masterPort)
	if err != nil {
		return nil, errors.Wrap(err, "error connecting to master")
	}

	err = writeToConn(conn, []string{"PING"});
	if err != nil {return nil, err}

	// second, send the REPLCONF commands to master
	err = writeToConn(conn, []string{"REPLCONF", "listening-port", *port})
	if err != nil {return nil, err}
	err = writeToConn(conn, []string{"REPLCONF", "capa", "psync2"})
	if err != nil {return nil, err}
	
	// third, complete hand shake with psync command to sync replica?
	err = writeToConn(conn, []string{"PSYNC", "?", "-1"})
	if err != nil {
		return nil, err
	}
	
	return conn, nil
}

func writeToConn(conn net.Conn, toSend []string) error {
	_, err := conn.Write(BulkStringArraySerialize(toSend))

	if err != nil {
		return errors.Wrap(err, "error messaging master: " + strings.Join(toSend, " "))
	}

	buffer := make([]byte, 3000)
	_, err = conn.Read(buffer)
	
	if err != nil {
		return errors.Wrap(err, "error getting response from master")
	}

	// sending worked out
	// TODO: can verify later what we got (basically should return a deserialized response)
	fmt.Println("I GOT FROM MASTER A:", string(buffer))
	sections := fullRESPDeserialize(string(buffer));

	if len(sections) == 0 || len(sections[0]) == 0 {
		return errors.New("NOTHIGN GOTTEN")
	}

	if strings.Split(sections[0][0], " ")[0] == "FULLRESYNC" {
		// if didn't get RDB, get it
		if len(sections) < 2 {
			buffer := make([]byte, 3000)
			_, err = conn.Read(buffer)
			
			if err != nil {
				return errors.Wrap(err, "error getting response from master")
			}

			sections = fullRESPDeserialize(string(buffer))
		} else {
			sections = sections[1:]
		}
	}

	// process each section after first
	for _, section := range sections[1:] {
		handleCommand(section, conn)
	}	

	return nil
}

func handleMasterConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Println("Master connected:", conn.RemoteAddr())

	// create a buffer of bytes to read the input into
	buffer := make([]byte, 1024)
	for {
		// read from connection
		n,err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Closing connection...")
				return
			} else {
				fmt.Println("Error reading from connection:", err)
				return
			}
		}
		
		sections := fullRESPDeserialize(string(buffer[:n]))
		for _, section := range sections {
			handleCommand(section, conn);
		}
		myRepOffset += n;
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Println("Client connected:", conn.RemoteAddr())

	// create a buffer of bytes to read the input into
	buffer := make([]byte, 1024)
	for {
		// read from connection
		n,err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Closing connection...")
				return
			} else {
				fmt.Println("Error reading from connection:", err)
				return
			}
		}
		
		sections := fullRESPDeserialize(string(buffer[:n]))
		for _, section := range sections {
			handleCommand(section, conn);
		}

	}
}

func handleCommand(parts []string, conn net.Conn) {
		// get deserialized commands + args
		fmt.Println(parts)

		// decide action to take based on first (should be command)
		if(len(parts) == 0) {
			fmt.Println("No commands passed - nothing to do")
		} else {
			command := strings.ToLower(parts[0])
			cmdFunc, exists := CmdMap[command]
			if exists {
				fmt.Println("Executing command:", command)
				cmdFunc(conn, parts[1:])

				// check if need to propoate this command to replicas
				_, canProp := PropCmds[command]
				if canProp {
					for _, replicaConn := range replicaConnections {
						(*replicaConn).Write(BulkStringArraySerialize(parts))
					}
				}
			} else {
				fmt.Println("Unknown command passed:", command)
			}
		}
}