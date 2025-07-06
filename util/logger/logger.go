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
	Trace
)

const logLevelEnv = "LOG_LEVEL"

var logLevel LogLevel
var fileLogger *log.Logger
var consoleLogger *log.Logger
var logFilePath string
var enabled bool = true
var fileEnabled bool = true

func init() {
	initLogger()

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
	case "TRACE":
		logLevel = Trace
	default:
		logLevel = Info
		Warnf("Unknown log level '%s', defaulting to INFO", envvar)
	}
}

// initLogger initializes the logger, creating a temporary log file and setting up console and file loggers.
func initLogger() {
	consoleLogger = log.New(os.Stderr, "", log.LstdFlags)

	tempFile, err := os.CreateTemp("", "app-*.log")
	if err != nil {
		consoleLogger.Printf("Failed to create temp log file: %v", err)
		return
	}

	logFilePath = tempFile.Name()
	consoleLogger.Printf("Logging to file: %s", logFilePath)

	fileLogger = log.New(tempFile, "", log.LstdFlags|log.Lmicroseconds)
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
	case Trace:
		return "TRACE"
	default:
		return "UNKNOWN"
	}
}

// Errorf prints an error message prefixed with "[ERROR] " and stops execution.
// After Errorf nothing will be executed anymore.
// A newline is added to the end of the message.
func Errorf(format string, v ...any) {
	logFormat := fmt.Sprintf("[ERROR] %s", format)
	if fileEnabled {
		fileLogger.Printf(logFormat, v...)
	}
	consoleLogger.Fatalf(logFormat, v...)
	assert.Never()
}

// Warnf prints a message prefixed with "[WARN] ".
// A newline is added to the end of the message.
func Warnf(format string, v ...any) {
	if !enabled {
		return
	}

	logFormat := fmt.Sprintf("[WARN] %s", format)
	if fileEnabled {
		fileLogger.Printf(logFormat, v...)
	}
	if logLevel >= Warn {
		consoleLogger.Printf(logFormat, v...)
	}
}

// Panicf acts similar to [Errorf] but panics.
// All deferred functions will execute and a stack trace is printed.
// Technically you can recover from the panic, but that's not intended use.
func Panicf(format string, v ...any) {
	logFormat := fmt.Sprintf("[ERROR] %s", format)
	if fileEnabled {
		fileLogger.Printf(logFormat, v...)
	}
	consoleLogger.Panicf(logFormat, v...)
	assert.Never()
}

// Infof prints an informational message prefixed with "[INFO] ".
// A newline is added to the end of the message.
func Infof(format string, v ...any) {
	if !enabled {
		return
	}

	logFormat := fmt.Sprintf("[INFO] %s", format)
	if fileEnabled {
		fileLogger.Printf(logFormat, v...)
	}
	if logLevel >= Info {
		consoleLogger.Printf(logFormat, v...)
	}
}

// Debugf prints a debug message prefixed with "[DEBUG] ".
// A newline is added to the end of the message.
func Debugf(format string, v ...any) {
	if !enabled {
		return
	}

	logFormat := fmt.Sprintf("[DEBUG] %s", format)
	if fileEnabled {
		fileLogger.Printf(logFormat, v...)
	}
	if logLevel >= Debug {
		consoleLogger.Printf(logFormat, v...)
	}
}

// Tracef prints a trace message prefixed with "[TRACE] ".
// A newline is added to the end of the message.
func Tracef(format string, v ...any) {
	if !enabled {
		return
	}

	logFormat := fmt.Sprintf("[TRACE] %s", format)
	if fileEnabled {
		fileLogger.Printf(logFormat, v...)
	}
	if logLevel >= Trace {
		consoleLogger.Printf(logFormat, v...)
	}
}

// GetLogFilePath returns the path to the current log file
func GetLogFilePath() string {
	return logFilePath
}

// SetEnable sets whether logging is enabled or not.
// Errors and panics will still be logged, but other log levels will not output anything if disabled.
func SetEnable(enable bool) {
	if enable {
		enabled = true
		Infof("--- LOGGING ENABLED ---")
	} else {
		Infof("--- LOGGING DISABLED ---")
		enabled = false
	}
}

func SetFileEnable(enable bool) {
	if enable {
		fileEnabled = true
		Infof("--- FILE LOGGING ENABLED ---")
	} else {
		fileEnabled = false
		Infof("--- FILE LOGGING DISABLED ---")
	}
}
