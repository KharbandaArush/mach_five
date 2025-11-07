package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Logger wraps standard logger with level support
type Logger struct {
	level  string
	logger *log.Logger
	file   *os.File
}

// NewLogger creates a new logger instance
func NewLogger(level, logPath string) (*Logger, error) {
	// Create log directory if it doesn't exist
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	logger := log.New(multiWriter, "", log.LstdFlags|log.Lmicroseconds)

	return &Logger{
		level:  strings.ToUpper(level),
		logger: logger,
		file:   file,
	}, nil
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func (l *Logger) shouldLog(level string) bool {
	levels := map[string]int{
		"DEBUG": 0,
		"INFO":  1,
		"WARN":  2,
		"ERROR": 3,
	}

	currentLevel, ok := levels[l.level]
	if !ok {
		currentLevel = 1 // Default to INFO
	}

	msgLevel, ok := levels[level]
	if !ok {
		return true
	}

	return msgLevel >= currentLevel
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.shouldLog("DEBUG") {
		l.logger.Printf("[DEBUG] "+format, v...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	if l.shouldLog("INFO") {
		l.logger.Printf("[INFO] "+format, v...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	if l.shouldLog("WARN") {
		l.logger.Printf("[WARN] "+format, v...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	if l.shouldLog("ERROR") {
		l.logger.Printf("[ERROR] "+format, v...)
	}
}

// JSON logs a JSON message
func (l *Logger) JSON(data interface{}) {
	// Simple JSON logging - in production, use proper JSON encoder
	l.Info("%+v", data)
}


