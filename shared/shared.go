package shared

import (
	"cadence/parser"
	"net"
)

const (
	DefaultPort          = "6380"
	MaxInstructionBuffer = 5
)

type ServerBasicInfo struct {
	IsReplica     bool
	MasterAddress string // empty string if not replica
	Port          string
	CurrentOffset int
}

var ServerInfo = ServerBasicInfo{}

func WriteToConn(conn net.Conn, message string) error {
	_, err := conn.Write(parser.BulkStringSerialize(message))
	return err
}
