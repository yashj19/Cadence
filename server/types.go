package server

import (
	"fmt"
	"net"
	"slices"
	"strings"

	"cadence/utils"
)

// SERVER_BASIC_INFO --------------------------------------------------------------------------
type ServerBasicInfo struct {
	IsReplica     bool
	MasterAddress string // empty string if not replica
	Port          string
	CurrentOffset int
}

// INSTRUCTION --------------------------------------------------------------------------------
type Instruction struct {
	Command string
	Args    []string
}

func (inst *Instruction) Validate() (bool, string) {
	commandInfo, exists := cmdMap[strings.ToUpper(inst.Command)]
	if !exists {
		return false, "Invalid command."
	} else if !commandInfo.Validate(inst.Args) {
		return false, "Invalid use of command."
	} else {
		return true, "Valid command use."
	}
}

func (inst *Instruction) Run(conn net.Conn) {
	fmt.Print("Running inst: ")
	inst.Print()
	valid, errorMsg := inst.Validate()
	if !valid {
		errorMsg = fmt.Sprintf("ERROR: %s", errorMsg)
		fmt.Println(errorMsg)
		utils.WriteToConn(conn, errorMsg)
	} else {
		executionFunc := cmdMap[strings.ToUpper(inst.Command)].Execute
		conn.Write(executionFunc(inst.Args, conn))

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
				replica.connection.Write(inst.Serialize())
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

func (inst *Instruction) Print() {
	fmt.Println(inst.Command + " " + strings.Join(inst.Args, " "))
}

func (inst *Instruction) String() string {
	ans := inst.Command
	if len(inst.Args) > 0 {
		ans += " " + strings.Join(inst.Args, " ")
	}
	return ans
}

func (inst *Instruction) Serialize() []byte {
	return utils.BulkStringArraySerialize(append([]string{inst.Command}, inst.Args...))
}

// like constructor for instruction struct
func NewInstruction(rawParts []string) Instruction {
	// TODO: make this smoother
	if len(rawParts) == 0 {
		return Instruction{}
	}
	return Instruction{Command: rawParts[0], Args: rawParts[1:]}
}

// RESPONSE -------------------------------------------------------------------------------------------------
type Response string

func (r Response) Print() {
	fmt.Println(string(r))
}

func (r Response) String() string {
	return string(r)
}

func NewResponse(rawParts []string) Response {
	return Response(strings.Join(rawParts, "\n"))
}

