package shared

import (
	"cadence/parser"
	"net"
)

const (
	DefaultPort = "6380"
)

var ServerInfo = struct {
	IsReplica     bool
	MasterHost    string
	MasterPort    string
	Port          string
	CurrentOffset int
}{
	IsReplica:     false,
	MasterHost:    "",
	MasterPort:    "",
	Port:          DefaultPort,
	CurrentOffset: 0,
}

func WriteToConn(conn net.Conn, message string) {
	conn.Write(parser.BulkStringSerialize(message))
}
