package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	logMu       sync.Mutex
	currentFile *os.File
	currentPath string
)

func getLogFilePath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	dir := filepath.Join(localAppData, "switchback")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "switchback.log")
}

func ensureLogFileLocked(logPath string) error {
	if currentFile != nil && currentPath == logPath {
		// Check rotation (if > 5MB)
		if fi, err := currentFile.Stat(); err == nil && fi.Size() > 5*1024*1024 {
			_ = currentFile.Close()
			currentFile = nil
			oldPath := logPath + ".old"
			_ = os.Remove(oldPath)
			_ = os.Rename(logPath, oldPath)
		}
	}

	if currentFile == nil || currentPath != logPath {
		if currentFile != nil {
			_ = currentFile.Close()
		}
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		currentFile = f
		currentPath = logPath
	}
	return nil
}

// Log writes a message with timestamp and level to the persistent log file handle.
func Log(level, format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()

	logPath := getLogFilePath()
	if err := ensureLogFileLocked(logPath); err != nil {
		return
	}

	msg := fmt.Sprintf(format, args...)
	entry := fmt.Sprintf("[%s] [%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), level, msg)
	_, _ = currentFile.WriteString(entry)
	_ = currentFile.Sync()
}

func Info(format string, args ...interface{}) {
	Log("INFO", format, args...)
}

func Debug(format string, args ...interface{}) {
	Log("DEBUG", format, args...)
}

func Warn(format string, args ...interface{}) {
	Log("WARN", format, args...)
}

func Error(format string, args ...interface{}) {
	Log("ERROR", format, args...)
}

// SanitizeLogLine redacts sensitive user paths before serving logs.
func SanitizeLogLine(line string) string {
	home, _ := os.UserHomeDir()
	if home != "" {
		line = strings.ReplaceAll(line, home, "~")
	}
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		line = strings.ReplaceAll(line, localAppData, "%LOCALAPPDATA%")
	}
	return line
}

// GetRecentLogs returns the last N non-blank sanitized lines from switchback.log.
func GetRecentLogs(maxLines int) []string {
	logMu.Lock()
	defer logMu.Unlock()

	if currentFile != nil {
		_ = currentFile.Sync()
	}

	logPath := getLogFilePath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		return []string{"[INFO] Log file created."}
	}

	rawLines := strings.Split(string(data), "\n")
	filtered := make([]string, 0, len(rawLines))
	for _, l := range rawLines {
		l = strings.TrimSpace(l)
		if l != "" {
			filtered = append(filtered, SanitizeLogLine(l))
		}
	}

	if len(filtered) <= maxLines {
		return filtered
	}
	return filtered[len(filtered)-maxLines:]
}
