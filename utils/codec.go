package utils

import (
	"strconv"
	"strings"
)

func SimpleStringSerialize(s string) []byte {
	return []byte("+" + s + "\r\n")
}

func BulkStringSerialize(s string) []byte {
	return []byte("$" + strconv.Itoa(len(s)) + "\r\n" + s + "\r\n")
}

func BulkStringArraySerialize(arr []string) []byte {
	temp := "*" + strconv.Itoa(len(arr)) + "\r\n"

	for _, s := range arr {
		temp += string(BulkStringSerialize(s))
	}
	return []byte(temp)
}

func RDBFileSerialize(binFile []byte) []byte {
	return []byte("$" + strconv.Itoa(len(binFile)) + "\r\n" + string(binFile))
}

func NilBulkString() []byte {
	return []byte("$-1\r\n")
}

func fullRESPDeserialize(serializedString string) [][]string {
	ans := [][]string{}
	return helper(serializedString, ans, 0)
}

func lengthExtractor(s string) (int, string, error) {
	lengthStr := ""
	var i int
	for i = 1; s[i] != '\r'; i++ {
		lengthStr += string(s[i])
	}
	n, err := strconv.Atoi(lengthStr)
	if err != nil {
		return 0, "", err
	}

	return n, s[i+2:], nil
}

// create a recursive helper to accumulate answer
func helper(s string, acc [][]string, offset int) [][]string {
	if len(s) < 1 {
		return acc
	}

	// either gonna start with + (simple string), $ (bulk string or file), * (array of resps)
	firstChar := s[0]
	switch firstChar {
	case '+':
		temp := ""
		var i int
		for i = 1; s[i] != '\r'; i++ {
			temp += string(s[i])
		}
		if offset == 0 {
			acc = append(acc, []string{temp})
		} else {
			offset--
			acc[len(acc)-1] = append(acc[len(acc)-1], temp)
		}
		return helper(s[i+2:], acc, offset)
	case '$':
		// extract length
		length, s, err := lengthExtractor(s)
		if err != nil {
			return acc
		}

		// extract string
		if length > len(s) {
			return acc
		}
		bulkString := s[:length]
		if offset == 0 {
			acc = append(acc, []string{bulkString})
		} else {
			offset--
			acc[len(acc)-1] = append(acc[len(acc)-1], bulkString)
		}

		// check if its an RDB file or bulk string
		if length == len(s) || s[length] != '\r' {
			return helper(s[length:], acc, offset)
		} else {
			return helper(s[length+2:], acc, offset)
		}
	case '*':
		// extract length of array
		length, s, err := lengthExtractor(s)
		if err != nil {
			return acc
		}

		// append empty acc, and pass length as offset
		acc = append(acc, []string{})
		return helper(s, acc, length)

	default:
		return acc
	}
}

func RESPDeserialize(s string) []string {

	// first *n\r\n
	// followed by n of the following
	// $m/r/n
	// make more efficient later

	parts := strings.Split(s, "\r\n")
	// starting from second part, and skipping by 1s are all the stuff we want
	ans := make([]string, (len(parts)-1)/2)
	for i := 2; i < len(parts); i += 2 {
		ans[(i-1)/2] = parts[i]
	}
	return ans
}

// later implement Serialize
