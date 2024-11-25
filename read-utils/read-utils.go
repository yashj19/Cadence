package readutils

import (
	"cadence/commands"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/pkg/errors"
)

func readRawDataFromConnection(dataChannel chan<- string, conn net.Conn) {
	buffer := make([]byte, 1024)
	for {
		// wait until a message comes in
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Println("CONNECTION_STATUS: Connection closed...")
				break
			} else {
				fmt.Println("ERROR: error reading from connection:", err)
				break
			}
		}
		dataChannel <- string(buffer[:n])
	}
	defer close(dataChannel) // close channel at end
}

func interpretRecievedBytes(dataChannel <-chan string, instChannel chan<- commands.Instruction) {
	var data = ""
	var i = 0
	var channelDead = false
	var getNextChars = func(n int) string {
		for len(data) < i+n {
			newData, ok := <-dataChannel
			channelDead = ok
			if len(data) > i {
				data = data[i:] + newData
			} else {
				data = newData
			}
			i = 0

			if !ok {
				return ""
			}
		}
		return data[i : i+n]
	}

	var lengthExtractor = func() (int, error) {
		lengthStr := ""
		t := getNextChars(1)
		for !channelDead && t[0] != '\r' {
			lengthStr += t
			t = getNextChars(1)
		}
		if channelDead {
			return -1, errors.New("channel died")
		}
		n, err := strconv.Atoi(lengthStr)
		if err != nil {
			return -1, err
		}

		return n, nil
	}

	// iterate over the data channel
	for {
		// either gonna start with + (simple string), $ (bulk string), * (array of bulk strings)
		firstChar := getNextChars(1)[0]
		if channelDead {
			return
		}
		switch firstChar {
		case '+':
			temp := ""
			t := getNextChars(1)
			for !channelDead && t[0] != '\r' {
				temp += t
				t = getNextChars(1)
			}
			getNextChars(1) // skip \n that should come after
			if channelDead {
				return
			}
			instChannel <- commands.NewInstruction([]string{temp})
		case '$':
			length, err := lengthExtractor()
			if err != nil {
				continue
			}
			bulkString := getNextChars(length)
			if channelDead {
				return
			}
			instChannel <- commands.NewInstruction([]string{bulkString})
		case '*':
			// extract length of array
			length, err := lengthExtractor()
			if err != nil {
				continue
			}
			arr := []string{}
			for i := 0; i < length; i++ {
				length, err := lengthExtractor()
				if err != nil {
					continue
				}
				bulkString := getNextChars(length)
				if channelDead {
					return
				}
				arr = append(arr, bulkString)
			}
			instChannel <- commands.NewInstruction(arr)
		default:
			continue
		}
	}
	// TODO: LATER VALIDATE BEFORE SENDING IN CHANNEL
}

func ReadFromConn(conn net.Conn) chan commands.Instruction {
	dataChannel := make(chan string)
	instChannel := make(chan commands.Instruction)
	go readRawDataFromConnection(dataChannel, conn)
	go interpretRecievedBytes(dataChannel, instChannel)
	return instChannel
}
