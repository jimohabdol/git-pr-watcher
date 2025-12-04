package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type LogLevel int

const (
	ERROR LogLevel = iota
	INFO
	DEBUG
	VERBOSE
)

type Logger struct {
	level  LogLevel
	logger *log.Logger
}

func New(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

func (l *Logger) Error(format string, v ...interface{}) {
	if l.level >= ERROR {
		l.logger.Printf("[ERROR] "+format, v...)
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	if l.level >= INFO {
		l.logger.Printf("[INFO] "+format, v...)
	}
}

func (l *Logger) Debug(format string, v ...interface{}) {
	if l.level >= DEBUG {
		l.logger.Printf("[DEBUG] "+format, v...)
	}
}

func (l *Logger) Verbose(format string, v ...interface{}) {
	if l.level >= VERBOSE {
		l.logger.Printf("[VERBOSE] "+format, v...)
	}
}

func (l *Logger) Progress(format string, v ...interface{}) {
	if l.level >= INFO {
		fmt.Printf("\r[PROGRESS] %-60s", fmt.Sprintf(format, v...))
	}
}

func (l *Logger) ProgressEnd() {
	if l.level >= INFO {
		fmt.Println()
	}
}

func ParseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "error":
		return ERROR
	case "info":
		return INFO
	case "debug":
		return DEBUG
	case "verbose":
		return VERBOSE
	default:
		return INFO
	}
}

var globalLogger *Logger

func Init(level LogLevel) {
	globalLogger = New(level)
}

func Get() *Logger {
	if globalLogger == nil {
		globalLogger = New(INFO)
	}
	return globalLogger
}

func Error(format string, v ...interface{}) {
	Get().Error(format, v...)
}

func Info(format string, v ...interface{}) {
	Get().Info(format, v...)
}

func Debug(format string, v ...interface{}) {
	Get().Debug(format, v...)
}

func Verbose(format string, v ...interface{}) {
	Get().Verbose(format, v...)
}

func Progress(format string, v ...interface{}) {
	Get().Progress(format, v...)
}

func ProgressEnd() {
	Get().ProgressEnd()
}
