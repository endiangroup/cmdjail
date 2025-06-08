package main

import (
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
)

var debug bool

func setLoggerToSyslog() error {
	logWriter, err := syslog.New(syslog.LOG_SYSLOG, "cmdjail")
	if err != nil {
		return fmt.Errorf("unable to set up syslog: %w", err)
	}
	log.SetOutput(logWriter)
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	return nil
}

func setLoggerToFile(path string) error {
	logFd, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("unable to create log file %s: %w", path, err)
	}
	log.SetOutput(logFd)
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	return nil
}

func printMsg(printTo io.Writer, msg string, args ...any) {
	fmt.Fprintf(printTo, msg+"\n", args...)
}

func logMsg(msg string, args ...any) {
	log.Printf(msg+"\n", args...)
}

func printLog(printTo io.Writer, msg string, args ...any) {
	printMsg(printTo, msg, args...)
	logMsg(msg+"\n", args...)
}

func printErr(printTo io.Writer, msg string, args ...any) {
	printMsg(printTo, "[error] "+msg, args...)
}

func logErr(msg string, args ...any) {
	logMsg("[error] "+msg, args...)
}

func logWarn(msg string, args ...any) {
	logMsg("[warn] "+msg, args...)
}

func printLogErr(printTo io.Writer, msg string, args ...any) {
	printLog(printTo, "[error] "+msg, args...)
}

func printLogWarn(printTo io.Writer, msg string, args ...any) {
	printLog(printTo, "[warn] "+msg, args...)
}

func printLogDebug(printTo io.Writer, msg string, args ...any) {
	if debug {
		printLog(printTo, "[debug] "+msg, args...)
	}
}
