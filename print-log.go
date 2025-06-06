package main

import (
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
)

var debug bool

func setLoggerToSyslog() {
	logWriter, err := syslog.New(syslog.LOG_SYSLOG, "cmdjail")
	if err != nil {
		printErr(os.Stderr, "unable to set logfile: %s", err.Error())
		os.Exit(1)
	}
	log.SetOutput(logWriter)
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

func printLog(printTo io.Writer, msg string, args ...any) {
	fmt.Fprintf(printTo, msg, args...)
	log.Printf(msg, args...)
}

func printErr(printTo io.Writer, msg string, args ...any) {
	fmt.Fprintf(printTo, "[error] "+msg, args...)
}

func logErr(msg string, args ...any) {
	log.Printf("[error] "+msg, args...)
}

func logWarn(msg string, args ...any) {
	log.Printf("[warn] "+msg, args...)
}

func printLogErr(printTo io.Writer, msg string, args ...any) {
	printLog(printTo, "[error] "+msg, args...)
}

func printLogDebug(printTo io.Writer, msg string, args ...any) {
	if debug {
		printLog(printTo, "[debug] "+msg, args...)
	}
}
