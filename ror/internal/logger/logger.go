package logger

import "log"

func Parent(format string, args ...interface{}) {
	log.Printf("[PARENT] "+format, args...)
}

func Child(format string, args ...interface{}) {
	log.Printf("[CHILD] "+format, args...)
}

func Warn(format string, args ...interface{}) {
	log.Printf("[WARN] "+format, args...)
}

func Info(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

func Error(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}
