package utils

import (
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/pkg/errors"
)

// type ConnData []string

func readRawDataFromConnection(rawDataChannel chan<- string, conn net.Conn) {
	buffer := make([]byte, 1024)
	for {
		// wait until a message comes in
		n, err := conn.Read(buffer)
		// fmt.Println("Recieved bytes:", string(buffer))
		if err != nil {
			if err == io.EOF {
				fmt.Println("CONNECTION_STATUS: Connection closed...")
				break
			} else {
				fmt.Println("ERROR: error reading from connection:", err)
				break
			}
		}
		rawDataChannel <- string(buffer[:n])
	}
	defer close(rawDataChannel) // close channel at end
}

func interpretRecievedBytes[T any](rawDataChannel <-chan string, dataChannel chan<- T, dataCtor func([]string) T) {
	defer close(dataChannel)
	var data = ""
	var i = 0
	var channelDead = false
	var getNextChars = func(n int) string {
		// fmt.Println("OH BOY I RAN")
		for len(data) < i+n {
			// fmt.Println("HELL NAH, NOT ENOUGH DATA")
			newData, ok := <-rawDataChannel
			// fmt.Println("I JUST GOT: ", newData, ok)
			channelDead = !ok
			if len(data) > i {
				// fmt.Println("YO WHATS UP")
				data = data[i:] + newData
			} else {
				// fmt.Println("YO WHATS DOWN")
				data = newData
			}
			i = 0

			if !ok {
				// fmt.Println("SOB SOB SOB")
				return ""
			}
		}
		// fmt.Println("IM SENDING BACK:", data[i:i+n])
		// i should be incremented to after whatever is sent back
		oldi := i
		i = i + n
		return data[oldi : oldi+n]
	}

	var lengthExtractor = func() (int, error) {
		// fmt.Println("YOU NEED SOME LENGTH")
		lengthStr := ""
		t := getNextChars(1)
		for !channelDead && t[0] != '\r' {
			// fmt.Println("t loooks like:", t)
			lengthStr += t
			t = getNextChars(1)
		}
		// fmt.Println("I got a full:", lengthStr)
		if channelDead {
			// fmt.Println("NO, TAKE ME INSTEAD")
			return -1, errors.New("channel died")
		}
		n, err := strconv.Atoi(lengthStr)
		if err != nil {
			return -1, err
		}
		// fmt.Println("IM GIVING YOU SOME LENGTH:", n)
		getNextChars(1) // take \n
		return n, nil
	}

	// iterate over the data channel
	for {
		// either gonna start with + (simple string), $ (bulk string), * (array of bulk strings)
		firstChar := getNextChars(1)
		if channelDead {
			return
		}
		switch firstChar[0] {
		case '+':
			// fmt.Println("Interpreting simple string.")
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
			// fmt.Println("Simple string is:", temp)
			dataChannel <- dataCtor([]string{temp})
		case '$':
			// fmt.Println("Interpreting bulk string.")
			length, err := lengthExtractor()
			if err != nil {
				continue
			}
			var bulkString string
			if length >= 0 {
				bulkString = getNextChars(length)
				getNextChars(2)
			} else if length == -1 {
				bulkString = "NIL"
			}
			if channelDead {
				return
			}
			// fmt.Println("Bulk string is:", bulkString)
			dataChannel <- dataCtor([]string{bulkString})
		case '*':
			// fmt.Println("Interpreting bulk string array.")

			// extract length of array
			arrLength, err := lengthExtractor()
			if err != nil {
				// fmt.Println("CRY CRY CRY")
				continue
			}
			arr := []string{}
			for i := 0; i < arrLength; i++ {
				getNextChars(1) // remove $ from way
				length, err := lengthExtractor()
				if err != nil {
					continue
				}
				bulkString := getNextChars(length)
				if channelDead {
					return
				}
				arr = append(arr, bulkString)
				// skip over \r\n
				getNextChars(2)
				if channelDead {
					return
				}
			}

			// fmt.Println("Bulk string array is:", arr)
			dataChannel <- dataCtor(arr)
		default:
			fmt.Println("I DONT KNOW WHAT KIND OF THING YOU GAVE ME :SOB:")
			continue
		}
	}
}

func ReadFromConn[T any](conn net.Conn, dataCtor func([]string) T) chan T {
	rawDataChannel := make(chan string)
	dataChannel := make(chan T)
	go readRawDataFromConnection(rawDataChannel, conn)
	go interpretRecievedBytes(rawDataChannel, dataChannel, dataCtor)
	return dataChannel
}

func WriteToConn(conn net.Conn, message string) error {
	_, err := conn.Write(BulkStringSerialize(message))
	return err
}
