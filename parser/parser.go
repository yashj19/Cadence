package parser

import (
	"strconv"
	"strings"
)

func SimpleStringSerialize(s string) []byte {
	return []byte("+" + s + "\r\n");
}

func BulkStringSerialize(s string) []byte {
	return []byte("$" + strconv.Itoa(len(s)) + "\r\n" + s + "\r\n");
}

func BulkStringArraySerialize(arr []string) []byte {
	temp := "*" + strconv.Itoa(len(arr)) + "\r\n"

	for _,s := range arr {
		temp += string(BulkStringSerialize(s))
	}
	return []byte(temp)
}

func NilBulkString() []byte {
	return []byte("$-1\r\n")
}

func RESPDeserialize(s string) []string {

	// first *n\r\n
	// followed by n of the following
	// $m/r/n
	// make more efficient later

	parts := strings.Split(s, "\r\n")
	// starting from second part, and skipping by 1s are all the stuff we want
	ans := make([]string, (len(parts) - 1)/2)
	for i := 2; i < len(parts); i += 2 {
		ans[(i - 1)/2] = parts[i];
	}
	return ans;
}
// later implement Serialize
