package commands

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"cadence/parser"
)

var CmdMap = map[string]func(net.Conn, []string){
	"ping": ping,
	"echo": echo,
	"get":  get,
	"set":  set,
}

type Entry struct {
	value string
	expiryTime time.Time
}

var cache = make(map[string]Entry)

func invalidArgs() {
	fmt.Println("Invalid number of arguments passed");
}

func ping(conn net.Conn, args []string) {
	conn.Write(parser.SimpleStringSerialize("PONG"))
}

func echo(conn net.Conn, args []string) {
	if len(args) < 1 {
		invalidArgs()
		return
	}
	fmt.Println(args)
	conn.Write(parser.BulkStringSerialize(args[0]))
}

func get(conn net.Conn, args []string) {
	if len(args) < 1 {
		invalidArgs()
		return
	}
	entry, exists := cache[args[0]]
	if exists && (entry.expiryTime.IsZero() || time.Now().Before(entry.expiryTime)) {
		conn.Write(parser.BulkStringSerialize(entry.value))
	} else {
		delete(cache, args[0])
		conn.Write(parser.NilBulkString())
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

	conn.Write(parser.SimpleStringSerialize("OK"))
}