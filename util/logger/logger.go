package logger

import (
	"fmt"
	"log"
	"os"

	"bjoernblessin.de/chatprotogol/util/assert"
)

type LogLevel int

const (
	NONE LogLevel = iota
	WARN
	INFO
	DEBUG
)

const LOG_LEVEL_ENV = "LOG_LEVEL"

var logLevel LogLevel

func init() {
	envvar, present := os.LookupEnv(LOG_LEVEL_ENV)
	if !present {
		logLevel = INFO
		return
	}

	switch envvar {
	case "NONE":
		logLevel = NONE
	case "WARN":
		logLevel = WARN
	case "INFO":
		logLevel = INFO
	case "DEBUG":
		logLevel = DEBUG
	default:
		logLevel = INFO
		Warnf("Unknown log level '%s', defaulting to INFO", envvar)
	}
}

// Errorf prints an error message prefixed with "[ERROR] " and stops execution.
// After Errorf nothing will be executed anymore.
// A newline is added to the end of the message.
func Errorf(format string, v ...any) {
	log.Fatalf(fmt.Sprintf("[ERROR] %s", format), v...)
	assert.Never()
}

// Warnf prints a message prefixed with "[WARN] ".
// A newline is added to the end of the message.
func Warnf(format string, v ...any) {
	if logLevel < WARN {
		return
	}
	log.Printf(fmt.Sprintf("[WARN] %s", format), v...)
}

// Panicf acts similar to [Errorf] but panics.
// All deferred functions will execute and a stack trace is printed.
// Technically you can recover from the panic, but that's not intended use.
func Panicf(format string, v ...any) {
	log.Panicf(fmt.Sprintf("[ERROR] %s", format), v...)
	assert.Never()
}

// Infof prints an informational message prefixed with "[INFO] ".
// A newline is added to the end of the message.
func Infof(format string, v ...any) {
	if logLevel < INFO {
		return
	}
	log.Printf(fmt.Sprintf("[INFO] %s", format), v...)
}

// Debugf prints a debug message prefixed with "[DEBUG] ".
// A newline is added to the end of the message.
func Debugf(format string, v ...any) {
	if logLevel < DEBUG {
		return
	}
	log.Printf(fmt.Sprintf("[DEBUG] %s", format), v...)
}
