package logger

import (
	"fmt"
	"log"

	"bjoernblessin.de/chatprotogol/util/assert"
)

// Errorf prints an error message prefixed with "[ERROR] " and stops execution.
// After Errorf nothing will be executed anymore.
func Errorf(format string, v ...any) {
	log.Fatalf(fmt.Sprintf("[ERROR] %s", format), v...)
	assert.Never()
}

// Warnf prints a message prefixed with "[WARN] ".
func Warnf(format string, v ...any) {
	log.Printf(fmt.Sprintf("[WARN] %s", format), v...)
}

// Panicf acts similar to [Errorf] but panics.
// All deferred functions will execute and a stack trace is printed.
// Technically you can recover from the panic, but that's not intended use.
func Panicf(format string, v ...any) {
	log.Panicf(fmt.Sprintf("[ERROR] %s", format), v...)
	assert.Never()
}
