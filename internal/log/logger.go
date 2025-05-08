package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	loggerInstance *Logger
	once           sync.Once
)

type Logger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	level       LogLevel
	output      io.Writer
	mu          sync.Mutex
}

func Init(logPath string, level LogLevel) {
	once.Do(func() {
		if logPath == "" {
			fmt.Println("日志文件为空")
		}
		var output io.Writer = os.Stdout
		if logPath != "" {
			file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatalf("无法打开日志文件: %v", err)
			}
			output = file
		}

		loggerInstance = &Logger{
			debugLogger: log.New(output, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
			infoLogger:  log.New(output, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
			warnLogger:  log.New(output, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile),
			errorLogger: log.New(output, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
			level:       level,
			output:      output,
		}
	})
}

func GetLogger() *Logger {
	if loggerInstance == nil {
		Init("", INFO)
	}
	return loggerInstance
}

func (l *Logger) Debug(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level <= DEBUG {
		l.debugLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level <= INFO {
		l.infoLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Warn(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level <= WARN {
		l.warnLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level <= ERROR {
		l.errorLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

func Debug(format string, v ...interface{}) {
	GetLogger().Debug(format, v...)
}

func Info(format string, v ...interface{}) {
	GetLogger().Info(format, v...)
}

func Warn(format string, v ...interface{}) {
	GetLogger().Warn(format, v...)
}

func Error(format string, v ...interface{}) {
	GetLogger().Error(format, v...)
}
