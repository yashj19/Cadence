package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"

	"cadence/commands"
	"cadence/shared"

	"github.com/pkg/errors"
)

func main() {

	// get and parse flag for which port it is
	port := flag.String("port", shared.DefaultPort, "the port at which to run the db")
	replicaOf := flag.String("replicaof", "none", "the host and port of master node that this is a replica of in the format host:port")
	flag.Parse()

	// TODO: do some validation of the flags

	// set basic server info
	shared.ServerInfo = shared.ServerBasicInfo{
		IsReplica:     *replicaOf != "",
		MasterAddress: *replicaOf,
		Port:          *port,
		CurrentOffset: 0,
	}

	fmt.Println("Starting server at port " + *port + "...")

	// bind to a port to listen for incoming TCP connections
	l, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		fmt.Println("Failed to bind to port " + *port)
		os.Exit(1)
	}

	// close binding after function exits
	defer l.Close()

	// if its a replica, first perform handshake with master
	if shared.ServerInfo.IsReplica {
		masterConn, instChannel, err := handshakeMaster()
		if err != nil {
			fmt.Println("ERROR: handshake failed, ", err)
			os.Exit(1)
		}
		go handleConnection(masterConn, instChannel)
	}

	// wait for an incoming TCP connection
	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		instChannel := readInstructionsFromConn(c)
		go handleConnection(c, instChannel)
	}
}

func readFromConnection(dataChannel chan<- string, conn net.Conn) {
	buffer := make([]byte, 1024)
	for {
		// wait until a message comes in
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Println("CONNECTION_STATUS: Connection closed...")
				break
			} else {
				fmt.Println("ERROR: error reading from connection:", err)
				break
			}
		}
		dataChannel <- string(buffer[:n])
	}
	defer close(dataChannel) // close channel at end
}

func interpretRecievedBytes(dataChannel <-chan string, instChannel chan<- commands.Instruction) {
	var data = ""
	var i = 0
	var channelDead = false
	var getNextChars = func(n int) string {
		for len(data) < i+n {
			newData, ok := <-dataChannel
			channelDead = ok
			if len(data) > i {
				data = data[i:] + newData
			} else {
				data = newData
			}
			i = 0

			if !ok {
				return ""
			}
		}
		return data[i : i+n]
	}

	var lengthExtractor = func() (int, error) {
		lengthStr := ""
		t := getNextChars(1)
		for !channelDead && t[0] != '\r' {
			lengthStr += t
			t = getNextChars(1)
		}
		if channelDead {
			return -1, errors.New("channel died")
		}
		n, err := strconv.Atoi(lengthStr)
		if err != nil {
			return -1, err
		}

		return n, nil
	}

	// iterate over the data channel
	for {
		// either gonna start with + (simple string), $ (bulk string), * (array of bulk strings)
		firstChar := getNextChars(1)[0]
		if channelDead {
			return
		}
		switch firstChar {
		case '+':
			temp := ""
			t := getNextChars(1)
			for !channelDead && t[0] != '\r' {
				temp += t
				t = getNextChars(1)
			}
			getNextChars(1) // skip \n that should come after
			if channelDead {
				return
			}
			instChannel <- commands.NewInstruction([]string{temp})
		case '$':
			length, err := lengthExtractor()
			if err != nil {
				continue
			}
			bulkString := getNextChars(length)
			if channelDead {
				return
			}
			instChannel <- commands.NewInstruction([]string{bulkString})
		case '*':
			// extract length of array
			length, err := lengthExtractor()
			if err != nil {
				continue
			}
			arr := []string{}
			for i := 0; i < length; i++ {
				length, err := lengthExtractor()
				if err != nil {
					continue
				}
				bulkString := getNextChars(length)
				if channelDead {
					return
				}
				arr = append(arr, bulkString)
			}
			instChannel <- commands.NewInstruction(arr)
		default:
			continue
		}
	}
	// TODO: LATER VALIDATE BEFORE SENDING IN CHANNEL
}

func readInstructionsFromConn(conn net.Conn) chan commands.Instruction {
	dataChannel := make(chan string)
	instChannel := make(chan commands.Instruction)
	go readFromConnection(dataChannel, conn)
	go interpretRecievedBytes(dataChannel, instChannel)
	return instChannel
}

func handshakeMaster() (net.Conn, chan commands.Instruction, error) {
	// first, create tcp connection with master and start accepting reads from it
	conn, err := net.Dial("tcp", shared.ServerInfo.MasterAddress)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error connecting to master")
	}
	instChannel := readInstructionsFromConn(conn)

	// send PING and check if PONG received
	err = shared.WriteToConn(conn, commands.Commands.STATUS)
	if err != nil {
		return nil, nil, err
	}
	response := <-instChannel
	if response.Command != commands.Responses.ALL_GOOD {
		return nil, nil, errors.New("ERROR: master did not respond with a PONG")
	}

	// second, send the REPL_SYNC command to master and check if full sync received
	err = shared.WriteToConn(conn, commands.Commands.REPLICA_SYNC)
	if err != nil {
		return nil, nil, err
	}
	response = <-instChannel
	if response.Command != commands.Commands.FULL_SYNC {
		return nil, nil, errors.New("ERROR: master did not respond with a FULLSYNC")
	}
	// TODO - actually do stuff with the RDB file later

	// last, handle rest of connection
	fmt.Println("Master connected:", conn.RemoteAddr())
	return conn, instChannel, nil
}

func handleConnection(conn net.Conn, instChannel chan commands.Instruction) {
	defer conn.Close()
	fmt.Println("Client connected:", conn.RemoteAddr())
	for inst := range instChannel {
		inst.Run(conn)
	}
}

// Features I want this DB to have (custom):
// get, set, simple status check (wassup), info, and sync
// replication master to node
// automatic fault tolerance
// reads spread across all secondary/primary (load balancing)
// horizontal sharding
// make it LRU Cache
// transactions?
// support various types: strings, numbers, arrays, booleans, objects (nested) - want to know type so need to be normal maybe? --- LATER
