package logger

import (
	"fmt"
	"log"
	"os"

	"bjoernblessin.de/chatprotogol/util/assert"
)

type LogLevel int

const (
	None LogLevel = iota
	Warn
	Info
	Debug
)

const logLevelEnv = "LOG_LEVEL"

var logLevel LogLevel

func init() {
	envvar, present := os.LookupEnv(logLevelEnv)
	if !present {
		logLevel = Info
		return
	}

	switch envvar {
	case "NONE":
		logLevel = None
	case "WARN":
		logLevel = Warn
	case "INFO":
		logLevel = Info
	case "DEBUG":
		logLevel = Debug
	default:
		logLevel = Info
		Warnf("Unknown log level '%s', defaulting to INFO", envvar)
	}
}

func SetLogLevel(level LogLevel) {
	logLevel = level
}

func GetLogLevel() LogLevel {
	return logLevel
}

func (l LogLevel) String() string {
	switch l {
	case None:
		return "NONE"
	case Warn:
		return "WARN"
	case Info:
		return "INFO"
	case Debug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// Errorf prints an error message prefixed with "[ERROR] " and stops execution.
// After Errorf nothing will be executed anymore.
// A newline is added to the end of the message.
func Errorf(format string, v ...any) {
	log.Fatalf(fmt.Sprintf("[ERROR] %s", format), v...)
	assert.Never()
}

// Warnf prints a message prefixed with "[_WARN] ".
// A newline is added to the end of the message.
func Warnf(format string, v ...any) {
	if logLevel < Warn {
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

// Infof prints an informational message prefixed with "[_INFO] ".
// A newline is added to the end of the message.
func Infof(format string, v ...any) {
	if logLevel < Info {
		return
	}
	log.Printf(fmt.Sprintf("[INFO] %s", format), v...)
}

// Debugf prints a debug message prefixed with "[_DEBUG] ".
// A newline is added to the end of the message.
func Debugf(format string, v ...any) {
	if logLevel < Debug {
		return
	}
	log.Printf(fmt.Sprintf("[DEBUG] %s", format), v...)
}
