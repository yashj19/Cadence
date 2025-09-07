package server

// import (
// 	"cadence/utils"
// 	"os"

// 	"github.com/pkg/errors"
// )

// const log1 = "log1.txt"
// const log2 = "log2.txt"

// var currLog = log1
// var notCurrLog = log2

// func flipCurrLog() {
// 	if currLog == log1 {
// 		currLog = log2
// 		notCurrLog = log1
// 	} else {
// 		currLog = log1
// 		notCurrLog = log2
// 	}
// }

// // what we want here
// // this is going to be an append only log
// // functions required:
// // append to log, compact log (runs infrequently), read log, encrypt log + return, create log based on encrypted log

// // TODO: keep the connection open so don't have do it a bunch of times
// func AppendToLog(inst utils.Instruction) error {
// 	file, err := os.OpenFile(currLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
// 	if err != nil {
// 		return errors.Wrap(err, "Failed to log.")
// 	}
// 	defer file.Close()

// 	_, err = file.WriteString(inst.ToString())
// 	return err
// }

// func CompactLog() error {
// 	file, err := os.Create(notCurrLog)
// 	// read the contents backwards so the ones that are now evicted aren't added
// 	// add the ones that aren't evicted
// 	// switch current file name
// }
