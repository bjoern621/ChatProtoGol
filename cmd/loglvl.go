package cmd

import (
	"fmt"
	"strings"

	"bjoernblessin.de/chatprotogol/util/logger"
)

// HandleLogLevel displays or sets the current log level.
// Usage: loglevel [NONE|WARN|INFO|DEBUG]
func HandleLogLevel(args []string) {
	if len(args) > 1 {
		fmt.Println("Usage: loglvl [NONE|WARN|INFO|DEBUG]")
		return
	}

	// If an argument is provided, try to set the log level
	if len(args) == 1 {
		levelStr := strings.ToUpper(args[0])
		var level logger.LogLevel
		switch levelStr {
		case "NONE":
			level = logger.None
		case "WARN":
			level = logger.Warn
		case "INFO":
			level = logger.Info
		case "DEBUG":
			level = logger.Debug
		case "TRACE":
			level = logger.Trace
		default:
			fmt.Printf("Invalid log level: %s\n", levelStr)
			return
		}
		logger.SetLogLevel(level)
		fmt.Printf("Log level set to %s\n", levelStr)
		return
	}

	// If no arguments, just display the current level
	currentLevel := logger.GetLogLevel()
	fmt.Printf("Current log level: %s\n", currentLevel.String())
}
