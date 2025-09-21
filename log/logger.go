package log

import (
	"log"
)

var isDebug = false

const (
	ENV_DEBUG_LOG_VAR = "REPL_ENABLE_DEBUG_LOGS"
)

func Info(format string, a ...any) {
	log.Printf("[INFO] "+format, a...)
}

func Error(format string, a ...any) {
	log.Printf("[ERROR] "+format, a...)
}

func Warn(format string, a ...any) {
	log.Printf("[Warn] "+format, a...)
}

func Debug(format string, a ...any) {
	if isDebug {
		log.Printf("[DEBUG] "+format, a...)
	}
}

func SetDebug(enable bool) {
	if enable {
		isDebug = enable
		log.Println("Debugging enabled")
	} 
}

