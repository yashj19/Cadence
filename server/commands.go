package server

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"cadence/utils"
)

// create cache
type Entry struct {
	value      string
	expiryTime time.Time
}

var cache = make(map[string]Entry)

type Replica struct {
	host       string
	port       string
	connection net.Conn
}

var replicas = []*Replica{}



//TODO: each conn.Write can return error, handle it
/*
Commands supported:

PING
INFO
ECHO value
GET key value
SET key value [PX number]
PRINT - prints the contents of the entire db

REPLSYNC
FULLSYNC rdb_file - RDB file encoded as bulk string

Note: anything in brackets means its optional.
*/

// defined an explicit struct so the command names can easily be changed to make it more customizable
var Commands = struct {
	STATUS       string
	INFO         string
	ECHO         string
	GET          string
	SET          string
	DELETE       string
	PRINT        string
	REPLICA_SYNC string
	FULL_SYNC    string
}{
	STATUS:       "PING",
	INFO:         "INFO",
	ECHO:         "ECHO",
	GET:          "GET",
	SET:          "SET",
	DELETE:       "DELETE",
	PRINT:        "PRINT",
	REPLICA_SYNC: "REPLSYNC",
	FULL_SYNC:    "FULLSYNC",
}

var Responses = struct {
	ALL_GOOD string
	OKAY     string
}{
	ALL_GOOD: "PONG",
	OKAY:     "OK",
}

// list of commands to propagate to replicas
var commandsToPropagate = []string{Commands.SET}

// map of commands to validation functions
var cmdValidate = map[string]func(args []string) bool{
	Commands.STATUS: func(args []string) bool { return len(args) == 0 },
	Commands.INFO:   func(args []string) bool { return len(args) == 0 },
	Commands.ECHO:   func(args []string) bool { return len(args) > 0 },
	Commands.GET:    func(args []string) bool { return len(args) == 1 },
	Commands.SET: func(args []string) bool {
		if len(args) < 2 {
			return false
		}
		exIndex := slices.Index(args, "PX")

		// if sent PX option, better be longer than 4 args and last two args should be: PX number
		if exIndex == -1 {
			return true
		} else if len(args) >= 4 && exIndex == len(args)-2 {
			_, err := strconv.Atoi(args[len(args)-1])
			return err == nil
		} else {
			return false
		}
	},
	Commands.REPLICA_SYNC: func(args []string) bool {
		return len(args) == 0
	},
	Commands.FULL_SYNC: func(args []string) bool {
		//TODO: type should be a file
		return len(args) == 1
	},
}

// TODO: upon replica contact, save its host and port info (to try connecting again in the cdase of an error)
var cmdRun = map[string]func(net.Conn, []string){
	Commands.STATUS: func(conn net.Conn, args []string) {
		conn.Write(utils.SimpleStringSerialize("PONG"))
	},
	Commands.INFO: func(conn net.Conn, args []string) {
		if ServerInfo.IsReplica {
			conn.Write(utils.BulkStringSerialize("role:slave")) //\nmaster_replid:" + replID + "\nmaster_repl_offset:" + strconv.Itoa(repOffset) + "\n"))
		} else {
			conn.Write(utils.BulkStringSerialize("role:master"))
		}
	},
	Commands.ECHO: func(conn net.Conn, args []string) {
		conn.Write(utils.BulkStringSerialize(strings.Join(args, " ")))
	},
	Commands.GET: func(conn net.Conn, args []string) {
		entry, exists := cache[args[0]]
		if exists && (entry.expiryTime.IsZero() || time.Now().Before(entry.expiryTime)) {
			conn.Write(utils.BulkStringSerialize(entry.value))
		} else {
			delete(cache, args[0])
			conn.Write(utils.NilBulkString())
		}
	},
	Commands.SET: func(conn net.Conn, args []string) {
		var n = len(args)

		if n < 4 || strings.ToUpper(args[len(args)-2]) != "PX" {
			cache[args[0]] = Entry{value: args[1]}
		} else {
			num, err := strconv.Atoi(args[3])
			if err != nil {
				// it fails, write back an error
				conn.Write(utils.BulkStringSerialize("ERROR: An error occurred reading the expiry, please try again."))
			}
			cache[args[0]] = Entry{value: args[1], expiryTime: time.Now().Add(time.Duration(num) * time.Millisecond)}
		}

		if !ServerInfo.IsReplica {
			conn.Write(utils.SimpleStringSerialize("OK"))
		}
	},
	Commands.REPLICA_SYNC: func(conn net.Conn, args []string) {
		// REPLICA handshake is going to only be simple handshake - replica sends ask to sync with port, master replies with RDB file (full resync)
		host, port, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil {
			utils.WriteToConn(conn, "ERROR: could not add replica, try again")
		} else {
			replicas = append(replicas, &Replica{host: host, port: port, connection: conn})
			conn.Write(utils.BulkStringArraySerialize([]string{"FULLSYNC", "file"}))
		}
	},
	Commands.FULL_SYNC: func(conn net.Conn, args []string) {
		// REPLICA handshake is going to only be simple handshake - replica sends ask to sync with port, master replies with RDB file
		// get the port, do something with it (store it)
		// return back full resync
		conn.Write(utils.BulkStringArraySerialize([]string{"FULLSYNC", ""}))
	},
}

type Instruction struct {
	Command string
	Args    []string
}

func (inst Instruction) Validate() (bool, string) {
	validationFunc, exists := cmdValidate[strings.ToUpper(inst.Command)]
	if !exists {
		return false, "Invalid command."
	} else if !validationFunc(inst.Args) {
		return false, "Invalid use of command."
	} else {
		return true, "Valid command use."
	}
}

func (inst Instruction) Run(conn net.Conn) {
	fmt.Print("Running inst: ")
	inst.Print()
	valid, errorMsg := inst.Validate()
	if !valid {
		errorMsg = fmt.Sprintf("ERROR: %s", errorMsg)
		fmt.Println(errorMsg)
		utils.WriteToConn(conn, errorMsg)
	} else {
		executionFunc := cmdRun[strings.ToUpper(inst.Command)]
		executionFunc(conn, inst.Args)

		// if need to propagate it, do a couple things:
		// - propagate to replicas
		// - increment your replica offset
		// - append to log - should do this first regardless
		// - only keep a log as long as the memory you would like to store - run compaction on it?
		//  - perhaps, when you evict stuff from the thing, you append the evication to the log as well
		// 					- and then you can run compaction on it
		if slices.Contains(commandsToPropagate, inst.Command) {
			fmt.Println("Propagate command to any replicas.")
			// iterate through all replicas, and propagate
			for i, replica := range replicas {
				fmt.Println("Propagating to replica #", i)
				// write to connection, but if doesn't work retry (TODO)
				replica.connection.Write(utils.BulkStringSerialize(inst.Command + " " + strings.Join(inst.Args, " ")))
				/**
				TODO:
				when make instruction better (e.g. args actually store type information), make instruction
				also store the original command passed, so don't have to reconstruct it (POF)
				**/
			}

		}
	}

	fmt.Println("Done.")
}

func (inst Instruction) Print() {
	fmt.Println(inst.Command + " " + strings.Join(inst.Args, " "))
}

func (inst Instruction) ToString() string {
	ans := inst.Command
	if len(inst.Args) > 0 {
		ans += " " + strings.Join(inst.Args, " ")
	}
	return ans
}

// like constructor for instruction struct
func NewInstruction(rawParts []string) Instruction {
	// TODO: make this durable so that can gracefully clean after 0 parts received
	if len(rawParts) == 0 {
		fmt.Println("HELL NAH, YOU EXPECTING ME TO CREATE AN INSTRUCTION WITH NOTHIN?!?")
		return Instruction{}
		// return Instruction{}, errors.New("No parts for instruction constructor passed.")
	}

	return Instruction{Command: rawParts[0], Args: rawParts[1:]}
}

// TODO: make a separate interface and make two things follow it - new instruction and response
