package server

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"cadence/constants"
	"cadence/lru"
	"cadence/utils"

	"github.com/pkg/errors"
)

var ServerInfo = ServerBasicInfo{}
var cache = lru.ShardedLRU{}

func main() {

	// get and parse flag for which port it is
	port := flag.String("port", constants.DefaultPort, "the port at which to run the db")
	replicaOf := flag.String("replicaof", "", "the host and port of master node that this is a replica of in the format host:port")
	flag.Parse()

	// TODO: do some validation of the flags

	// set basic server info
	ServerInfo = ServerBasicInfo{
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

	// instantiate cache
	cache = lru.NewShardedLRU(constants.CAPACITY_PER_SHARD, constants.SHARD_COUNT)
	defer cache.Cleanup()

	// start a go routine to do snapshot every 5 minutes
	t := time.NewTicker(constants.SNAPSHOT_INTERVAL)
	snapshotStop := make(chan struct{})
    go func() {
        defer t.Stop()
        for {
            select {
            case <-t.C:
				cache.Snapshot("snapshot")
            case <-snapshotStop:
                return
            }
        }
    }()
	defer close(snapshotStop)

	// if its a replica, first perform handshake with master
	if ServerInfo.IsReplica {
		masterConn, instChannel, err := handshakeMaster()
		if err != nil {
			fmt.Println("ERROR: master handshake failed, ", err)
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
		instChannel := utils.ReadFromConn(c, NewInstruction)
		go handleConnection(c, instChannel)
	}
}

// expects a "RESPONSE" once and then an "INSTRUCTION"
func handshakeMaster() (net.Conn, chan Instruction, error) {
	fmt.Println("Commencing handshake with master, at remote address: ", ServerInfo.MasterAddress)

	// first, create tcp connection with master and start accepting reads from it
	conn, err := net.Dial("tcp", ServerInfo.MasterAddress)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error connecting to master")
	}
	instChannel := utils.ReadFromConn(conn, NewInstruction)

	// send PING and check if PONG received
	err = utils.WriteToConn(conn, Commands.STATUS)
	if err != nil {
		return nil, nil, err
	}
	response := <-instChannel
	if response.Command != Responses.ALL_GOOD {
		return nil, nil, errors.New("ERROR: master did not respond with a PONG")
	}

	// second, send the REPL_SYNC command to master and check if full sync received
	err = utils.WriteToConn(conn, Commands.REPLICA_SYNC)
	if err != nil {
		return nil, nil, err
	}
	response = <-instChannel
	if response.Command != Commands.FULL_SYNC {
		return nil, nil, errors.New("ERROR: master did not respond with a FULLSYNC")
	}
	// TODO - actually do stuff with the RDB file later

	// last, handle rest of connection
	fmt.Println("Master connected:", conn.RemoteAddr())
	return conn, instChannel, nil
}

// expects ONLY INSTRUCTIONS
func handleConnection(conn net.Conn, instChannel chan Instruction) {
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
