package commands

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"cadence/parser"
	"cadence/shared"
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

/*
Commands supported:

PING
INFO
ECHO value
GET key value
SET key value [EX number]
PRINT - prints the contents of the entire db

REPLSYNC
FULLSYNC rdb_file - RDB file encoded as bulk string

Note: anything in brackets means its optional.
*/

// defined an explicit struct so the command names can easily be changed to make it more customizable
var commands = struct {
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

// list of commands to propagate to replicas
var commandsToPropagate = []string{commands.SET}

// map of commands to validation functions
var cmdValidate = map[string]func(args []string) bool{
	commands.STATUS: func(args []string) bool { return len(args) == 0 },
	commands.INFO:   func(args []string) bool { return len(args) == 0 },
	commands.ECHO:   func(args []string) bool { return len(args) > 0 },
	commands.GET:    func(args []string) bool { return len(args) == 1 },
	commands.SET: func(args []string) bool {
		if len(args) < 2 {
			return false
		}
		exIndex := slices.Index(args, "EX")

		// if sent EX option, better be longer than 4 args and last two args should be: EX number
		if exIndex == -1 {
			return true
		} else if len(args) >= 4 && exIndex == len(args)-2 {
			_, err := strconv.Atoi(args[len(args)-1])
			return err == nil
		} else {
			return false
		}
	},
	commands.REPLICA_SYNC: func(args []string) bool {
		return len(args) == 0
	},
	commands.FULL_SYNC: func(args []string) bool {
		//TODO: type should be a file
		return len(args) == 1
	},
}

// TODO: upon replica contact, save its host and port info (to try connecting again in the cdase of an error)
var cmdRun = map[string]func(net.Conn, []string){
	commands.STATUS: func(conn net.Conn, args []string) {
		conn.Write(parser.SimpleStringSerialize("PONG"))
	},
	commands.INFO: func(conn net.Conn, args []string) {
		if shared.ServerInfo.IsReplica {
			conn.Write(parser.BulkStringSerialize("role:slave")) //\nmaster_replid:" + replID + "\nmaster_repl_offset:" + strconv.Itoa(repOffset) + "\n"))
		} else {
			conn.Write(parser.BulkStringSerialize("role:master"))
		}
	},
	commands.ECHO: func(conn net.Conn, args []string) {
		conn.Write(parser.BulkStringSerialize(strings.Join(args, " ")))
	},
	commands.GET: func(conn net.Conn, args []string) {
		entry, exists := cache[args[0]]
		if exists && (entry.expiryTime.IsZero() || time.Now().Before(entry.expiryTime)) {
			conn.Write(parser.BulkStringSerialize(entry.value))
		} else {
			delete(cache, args[0])
			conn.Write(parser.NilBulkString())
		}
	},
	commands.SET: func(conn net.Conn, args []string) {
		var n = len(args)

		if n < 4 || args[len(args)-2] != "PX" {
			cache[args[0]] = Entry{value: args[1]}
		} else {
			num, err := strconv.Atoi(args[3])
			if err != nil {
				// it fails, write back an error
				conn.Write(parser.BulkStringSerialize("ERROR: An error occurred reading the expiry, please try again."))
			}
			cache[args[0]] = Entry{value: args[1], expiryTime: time.Now().Add(time.Duration(num) * time.Millisecond)}
		}

		if !shared.ServerInfo.IsReplica {
			conn.Write(parser.SimpleStringSerialize("OK"))
		}
	},
	commands.REPLICA_SYNC: func(conn net.Conn, args []string) {
		// REPLICA handshake is going to only be simple handshake - replica sends ask to sync with port, master replies with RDB file (full resync)
		host, port, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil {
			shared.WriteToConn(conn, "ERROR: could not add replica, try again")
		} else {
			replicas = append(replicas, &Replica{host: host, port: port, connection: conn})
			conn.Write(parser.BulkStringArraySerialize([]string{"FULLSYNC", "file"}))
		}
	},
	commands.FULL_SYNC: func(conn net.Conn, args []string) {
		// REPLICA handshake is going to only be simple handshake - replica sends ask to sync with port, master replies with RDB file
		// get the port, do something with it (store it)
		// return back full resync
		conn.Write(parser.BulkStringArraySerialize([]string{"FULLSYNC", ""}))
	},
}

type Instruction struct {
	Command string
	Args    []string
}

func (inst Instruction) Validate() (bool, string) {
	validationFunc, exists := cmdValidate[inst.Command]
	if !exists {
		return false, "Invalid command."
	} else if !validationFunc(inst.Args) {
		return false, "Invalid use of command."
	} else {
		return true, "Valid command use."
	}
}

func (inst Instruction) Run(conn net.Conn) {
	valid, errorMsg := inst.Validate()
	if !valid {
		errorMsg = fmt.Sprintf("ERROR: %s", errorMsg)
		fmt.Println(errorMsg)
		shared.WriteToConn(conn, errorMsg)
	} else {
		executionFunc := cmdRun[inst.Command]
		executionFunc(conn, inst.Args)

		// if need to propagate it, do that as well
		if slices.Contains(commandsToPropagate, inst.Command) {
			// iterate through all replicas, and propagate
			for _, replica := range replicas {
				// write to connection, but if doesn't work retry (TODO)
				replica.connection.Write(parser.BulkStringSerialize(inst.Command + " " + strings.Join(inst.Args, " ")))
				/**
				when make instruction better (e.g. args actually store type information), make instruction
				also store the original command passed, so don't have to reconstruct it (POF)
				**/
			}

		}
	}
}

// like constructor for instruction struct
func NewInstruction(rawCommand string) Instruction {
	parts := strings.Split(rawCommand, " ")
	return Instruction{Command: parts[0], Args: parts[1:]}
}
