package server

import (
	"net"
	"os"
	"slices"
	"strconv"
	"strings"

	"cadence/utils"
)

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

// Command struct
type CommandInfo struct {
	DocString string
	Execute  func(args []string, conn net.Conn) []byte
	Validate func(args []string) bool
}

// map of commands to CommandInfo
var cmdMap = map[string]CommandInfo{
	Commands.STATUS: {
		DocString: "Ping the server",
		Execute: func(args []string, conn net.Conn) []byte {
			return utils.SimpleStringSerialize(Responses.ALL_GOOD)
		},
		Validate: func(args []string) bool {
			return len(args) == 0
		},
	},
	Commands.INFO: {
		DocString: "Get information about the server",
		Execute: func(args []string, conn net.Conn) []byte {
			if ServerInfo.IsReplica {
				return utils.BulkStringSerialize("role:slave")//\nmaster_replid:" + replID + "\nmaster_repl_offset:" + strconv.Itoa(repOffset) + "\n"))
			}
			return utils.BulkStringSerialize("role:master")
		},
		Validate: func(args []string) bool {
			return len(args) == 0
		},
	},
	Commands.ECHO: {
		DocString: "Echo the given message",
		Execute: func(args []string, conn net.Conn) []byte {
			return utils.BulkStringSerialize(strings.Join(args, " "))
		},
		Validate: func(args []string) bool {
			return len(args) > 0
		},
	},
	Commands.GET: {
		DocString: "Get the value of a key",
		Execute: func(args []string, conn net.Conn) []byte {
			value, exists := cache.Get(args[0])
			if exists {
				return utils.BulkStringSerialize(value)
			}
			return utils.NilBulkString()
		},
		Validate: func(args []string) bool {
			return len(args) == 1
		},
	},
	Commands.SET: {
		DocString: "Set the value of a key",
		Execute: func(args []string, conn net.Conn) []byte {
			var n = len(args)

			if n < 4 || strings.ToUpper(args[len(args)-2]) != "PX" {
				cache.Set(args[0], args[1], -1)
			} else {
				duration, err := strconv.Atoi(args[3])
				if err != nil {
					// it fails, write back an error
					return utils.BulkStringSerialize("ERROR: An error occurred reading the expiry, please try again.")
				}
				cache.Set(args[0], args[1], duration)
			}

			if !ServerInfo.IsReplica {
				return utils.SimpleStringSerialize(Responses.OKAY)
			}
			return nil
		},
		Validate: func(args []string) bool {
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
	},
	Commands.DELETE: {
		DocString: "Delete entry from cache",
		Execute: func(args []string, conn net.Conn) []byte {
			cache.Delete(args[0])
			if !ServerInfo.IsReplica {
				return utils.SimpleStringSerialize(Responses.OKAY)
			}
			return nil
		},
		Validate: func(args []string) bool {
			return len(args) == 1
		},
	},
	Commands.REPLICA_SYNC: {
		DocString: "Synchronize with a replica",
		Execute: func(args []string, conn net.Conn) []byte {
			// REPLICA handshake is going to only be simple handshake - replica sends ask to sync with port, master replies with RDB file (full resync)
			host, port, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err != nil {
				return utils.BulkStringArraySerialize([]string{"ERROR: could not add replica, try again"})
			} else {
				replicas = append(replicas, &Replica{host: host, port: port, connection: conn})
				data, err := os.ReadFile("snapshot.txt")
				if err != nil {
					return utils.BulkStringArraySerialize([]string{"ERROR: could not add replica, try again"})
				}
				return utils.BulkStringArraySerialize([]string{"FULLSYNC", string(data)}) // TODO: make this more efficient
			}
		},
		Validate: func(args []string) bool {
			return len(args) == 0
		},
	},
	Commands.FULL_SYNC: {
		DocString: "Perform a full synchronization",
		Execute: func(args []string, conn net.Conn) []byte {
			// REPLICA handshake is going to only be simple handshake - replica sends ask to sync with port, master replies with RDB file
			// get the port, do something with it (store it)
			// return back full resync - WHAT?
			return utils.BulkStringArraySerialize([]string{"FULLSYNC", ""})
		},
		Validate: func(args []string) bool {
			//TODO: type should be a file
			return len(args) == 1
		},
	},
}

