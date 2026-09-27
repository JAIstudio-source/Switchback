package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logMu sync.Mutex
)

func getLogFilePath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	dir := filepath.Join(localAppData, "focusmgr")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "focusmgr.log")
}

// Log writes a message with timestamp and level to the log file.
func Log(level, format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()

	logPath := getLogFilePath()

	// Check rotation (if > 5MB)
	if fi, err := os.Stat(logPath); err == nil && fi.Size() > 5*1024*1024 {
		oldPath := logPath + ".old"
		_ = os.Remove(oldPath)
		_ = os.Rename(logPath, oldPath)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	msg := fmt.Sprintf(format, args...)
	entry := fmt.Sprintf("[%s] [%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), level, msg)
	_, _ = f.WriteString(entry)
}

func Info(format string, args ...interface{}) {
	Log("INFO", format, args...)
}

func Debug(format string, args ...interface{}) {
	Log("DEBUG", format, args...)
}

func Error(format string, args ...interface{}) {
	Log("ERROR", format, args...)
}
