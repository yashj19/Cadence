package commands

import (
	"fmt"
	"net"

	"cadence/parser"
)

var CmdMap = map[string]func(net.Conn, []string){
	"ping": ping,
	"echo": echo,
	"get":  get,
	"set":  set,
}

var simpleMap = make(map[string]string)

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
	value, exists := simpleMap[args[0]]
	if exists {
		conn.Write(parser.BulkStringSerialize(value))
	} else {
		conn.Write(parser.NilBulkString())
	}
}

func set(conn net.Conn, args []string) {
	if len(args) < 2 {
		invalidArgs()
		return
	}
	simpleMap[args[0]] = args[1] 
	conn.Write(parser.SimpleStringSerialize("OK"))
}