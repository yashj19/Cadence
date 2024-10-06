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

// Entry struct {
// 	value any
// 	expiryTime time.TIME
// }

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
	var n = len(args)
	if n < 2 {
		invalidArgs()
		return
	}
	simpleMap[args[0]] = args[1] 

	// if expiry also set, handle that
	if n > 2 {
		if strings.ToLower(args[2]) == "px" {
			if n < 4 {
				invalidArgs()
				return
			}

			num, err := strconv.Atoi(args[3])
			if err != nil {
				fmt.Println("YOU DIDNT GIVE ME A NUMBER FOR EXPIRATION TIME")
				return
			}

			go expireKey(args[0], num);
		}
	}

	conn.Write(parser.SimpleStringSerialize("OK"))
}

func expireKey(key string, millis int) {
	time.Sleep(time.Duration(millis) * time.Millisecond)
	delete(simpleMap, key)
}