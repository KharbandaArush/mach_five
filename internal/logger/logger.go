package logger

import (
	"fmt"
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

	// Write only to file - systemd will capture stdout/stderr separately
	// This prevents duplicate logs (logger writes to file, systemd also captures stdout)
	logger := log.New(file, "", log.LstdFlags|log.Lmicroseconds)

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
		l.logger.Printf("🔍 [DEBUG] "+format, v...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	if l.shouldLog("INFO") {
		l.logger.Printf("ℹ️  [INFO] "+format, v...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	if l.shouldLog("WARN") {
		l.logger.Printf("⚠️  [WARN] "+format, v...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	if l.shouldLog("ERROR") {
		l.logger.Printf("❌ [ERROR] "+format, v...)
	}
}

// Success logs a success message
func (l *Logger) Success(format string, v ...interface{}) {
	if l.shouldLog("INFO") {
		l.logger.Printf("✅ [SUCCESS] "+format, v...)
	}
}

// JSON logs a JSON message
func (l *Logger) JSON(data interface{}) {
	// Simple JSON logging - in production, use proper JSON encoder
	l.Info("%+v", data)
}

// Table logs data in a formatted table
func (l *Logger) Table(headers []string, rows [][]string) {
	if !l.shouldLog("INFO") {
		return
	}
	
	l.logger.Println("")
	l.logger.Println("┌" + strings.Repeat("─", 100) + "┐")
	
	// Print headers
	headerRow := "│"
	for i, header := range headers {
		width := 20
		if i == len(headers)-1 {
			width = 100 - len(headerRow) - 1
		}
		headerRow += fmt.Sprintf(" %-*s │", width-2, truncate(header, width-2))
		if len(headerRow) >= 100 {
			break
		}
	}
	l.logger.Println(headerRow)
	
	// Print separator
	l.logger.Println("├" + strings.Repeat("─", 100) + "┤")
	
	// Print rows
	for _, row := range rows {
		rowStr := "│"
		for i, cell := range row {
			if i >= len(headers) {
				break
			}
			width := 20
			if i == len(headers)-1 {
				width = 100 - len(rowStr) - 1
			}
			rowStr += fmt.Sprintf(" %-*s │", width-2, truncate(cell, width-2))
			if len(rowStr) >= 100 {
				break
			}
		}
		l.logger.Println(rowStr)
	}
	
	l.logger.Println("└" + strings.Repeat("─", 100) + "┘")
	l.logger.Println("")
}

// TableSimple logs a simple 2-column table
func (l *Logger) TableSimple(title string, data map[string]string) {
	if !l.shouldLog("INFO") {
		return
	}
	
	l.logger.Println("")
	l.logger.Printf("╔══════════════════════════════════════════════════════════════╗")
	l.logger.Printf("║ %-60s ║", truncate(title, 60))
	l.logger.Printf("╠══════════════════════════════════════════════════════════════╣")
	
	maxKeyLen := 0
	for k := range data {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	}
	if maxKeyLen > 30 {
		maxKeyLen = 30
	}
	
	for k, v := range data {
		key := truncate(k, maxKeyLen)
		val := truncate(v, 60-maxKeyLen-3)
		l.logger.Printf("║ %-*s : %-*s ║", maxKeyLen, key, 60-maxKeyLen-3, val)
	}
	
	l.logger.Printf("╚══════════════════════════════════════════════════════════════╝")
	l.logger.Println("")
}

// Section logs a section header
func (l *Logger) Section(title string) {
	if !l.shouldLog("INFO") {
		return
	}
	l.logger.Println("")
	l.logger.Printf("╔══════════════════════════════════════════════════════════════╗")
	l.logger.Printf("║ %-60s ║", truncate(title, 60))
	l.logger.Printf("╚══════════════════════════════════════════════════════════════╝")
}

// IsDebug returns true if logger is in DEBUG mode
func (l *Logger) IsDebug() bool {
	return l.level == "DEBUG"
}

// truncate truncates a string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}


