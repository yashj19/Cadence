package main

import (
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

var CmdMap = map[string]func(net.Conn, []string){
	"ping": ping,
	"echo": echo,
	"get":  get,
	"set":  set,
	"info": info,
	"replconf": replconf,
	"psync": psync,
}

var PropCmds = map[string]bool {
	"set": true,
}

type Entry struct {
	value string
	expiryTime time.Time
}

var cache = make(map[string]Entry)
var replicaConnections = []*net.Conn{}

func invalidArgs() {
	fmt.Println("Invalid number of arguments passed");
}

func ping(conn net.Conn, args []string) {
	// THIS WONT LET YOU PING REPLICAS TO CHECK FOR HEALTH
	if masterHost == "" {
		conn.Write(SimpleStringSerialize("PONG"))
	}
}

func echo(conn net.Conn, args []string) {
	if len(args) < 1 {
		invalidArgs()
		return
	}
	fmt.Println(args)
	conn.Write(BulkStringSerialize(args[0]))
}

func get(conn net.Conn, args []string) {
	if len(args) < 1 {
		invalidArgs()
		return
	}
	entry, exists := cache[args[0]]
	if exists && (entry.expiryTime.IsZero() || time.Now().Before(entry.expiryTime)) {
		conn.Write(BulkStringSerialize(entry.value))
	} else {
		delete(cache, args[0])
		conn.Write(NilBulkString())
	}
}

func set(conn net.Conn, args []string) {
	var n = len(args)

	if n == 2 {
		cache[args[0]] = Entry{value: args[1]}
	} else if n == 4 && strings.ToLower(args[2]) == "px" {
		num, err := strconv.Atoi(args[3])
		if err != nil {
			fmt.Println("YOU DIDNT GIVE ME A NUMBER FOR EXPIRATION TIME")
			return
		}
		cache[args[0]] = Entry{value: args[1], expiryTime: time.Now().Add(time.Duration(num) * time.Millisecond)}
	} else {
		invalidArgs()
		return
	}

	if masterHost == "" {conn.Write(SimpleStringSerialize("OK"))}
}

func info(conn net.Conn, args []string) {
	if len(args) == 1 && args[0] == "replication" {
		if masterHost != "" {
			conn.Write(BulkStringSerialize("role:slave"));
		} else {
			conn.Write(BulkStringSerialize("role:master\nmaster_replid:" + replID + "\nmaster_repl_offset:" + strconv.Itoa(repOffset) + "\n"));
		}
	} else {
		invalidArgs()
	}
}

func replconf(conn net.Conn, args[]string) {

	if len(args) >= 1 && args[0] == "GETACK" {
		conn.Write(BulkStringArraySerialize([]string{"REPLCONF", "ACK", strconv.Itoa(myRepOffset)}))
	} else {
		conn.Write(SimpleStringSerialize("OK"))
	}
}

func psync(conn net.Conn, args[]string) {
	conn.Write(SimpleStringSerialize("FULLRESYNC " + replID + " " + strconv.Itoa(repOffset)))
	
	data := "UkVESVMwMDEx+glyZWRpcy12ZXIFNy4yLjD6CnJlZGlzLWJpdHPAQPoFY3RpbWXCbQi8ZfoIdXNlZC1tZW3CsMQQAPoIYW9mLWJhc2XAAP/wbjv+wP9aog=="
	binaryFile, err := base64.StdEncoding.DecodeString(string(data))

	if err != nil {
		fmt.Println("ERROR CONVERTING FILE")
		return
	}

	conn.Write(RDBFileSerialize(binaryFile))

	// now that the connection has been established to the replica, add this to the list of all replicas of this dude
	replicaConnections = append(replicaConnections, &conn)
}