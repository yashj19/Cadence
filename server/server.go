package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"cadence/commands"
	readutils "cadence/read-utils"
	"cadence/shared"

	"github.com/pkg/errors"
)

func main() {

	// get and parse flag for which port it is
	port := flag.String("port", shared.DefaultPort, "the port at which to run the db")
	replicaOf := flag.String("replicaof", "", "the host and port of master node that this is a replica of in the format host:port")
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
		instChannel := readutils.ReadFromConn(c)
		go handleConnection(c, instChannel)
	}
}

func handshakeMaster() (net.Conn, chan commands.Instruction, error) {
	fmt.Println("Commencing handshake with master, at remote address: ", shared.ServerInfo.MasterAddress)

	// first, create tcp connection with master and start accepting reads from it
	conn, err := net.Dial("tcp", shared.ServerInfo.MasterAddress)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error connecting to master")
	}
	instChannel := readutils.ReadFromConn(conn)

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
// replication master to node (NEED)
// automatic fault tolerance
// reads spread across all secondary/primary (load balancing) (NEED)
// horizontal sharding
// make it LRU Cache (NEED)
// logging (NEED)
// transactions?
// support various types: strings, numbers, arrays, booleans, objects (nested) - want to know type so need to be normal maybe? --- LATER
