package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxLogFileSize   = 10 * 1024 * 1024 // 10 MB per file
	maxLogFiles      = 5                // current + rotated copies (~50 MB)
	logRetentionDays = 30
	logFilePrefix    = "tarr-annunciator_"
	logFileSuffix    = ".log"
)

type rotatingFileWriter struct {
	mu      sync.Mutex
	dir     string
	file    *os.File
	path    string
	size    int64
	maxSize int64
}

func newRotatingFileWriter(dir string) (*rotatingFileWriter, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	w := &rotatingFileWriter{
		dir:     dir,
		maxSize: maxLogFileSize,
	}
	if err := w.rotateLocked(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil || w.size+int64(len(p)) > w.maxSize {
		if err := w.rotateLocked(); err != nil && w.file == nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *rotatingFileWriter) Path() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.path
}

func (w *rotatingFileWriter) rotateLocked() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}

	path := uniqueLogPath(w.dir)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	info, statErr := file.Stat()
	size := int64(0)
	if statErr == nil {
		size = info.Size()
	}

	w.file = file
	w.path = path
	w.size = size
	_ = pruneLogFiles(w.dir, w.path)
	return nil
}

func uniqueLogPath(dir string) string {
	ts := time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(dir, logFilePrefix+ts+logFileSuffix)
	for i := 1; fileExists(path); i++ {
		path = filepath.Join(dir, fmt.Sprintf("%s%s_%d%s", logFilePrefix, ts, i, logFileSuffix))
	}
	return path
}

func isAppLogFile(name string) bool {
	return strings.HasPrefix(name, logFilePrefix) && strings.HasSuffix(name, logFileSuffix)
}

func pruneLogFiles(dir, currentPath string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read logs directory: %v", err)
	}

	type logInfo struct {
		path    string
		modTime time.Time
		size    int64
	}

	cutoff := time.Now().AddDate(0, 0, -logRetentionDays)
	var kept []logInfo
	currentAbs, _ := filepath.Abs(currentPath)

	for _, entry := range entries {
		if entry.IsDir() || !isAppLogFile(entry.Name()) {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		abs, _ := filepath.Abs(path)
		if abs != "" && abs == currentAbs {
			continue
		}

		if info.ModTime().Before(cutoff) || info.Size() > maxLogFileSize {
			_ = os.Remove(path)
			continue
		}

		kept = append(kept, logInfo{path: path, modTime: info.ModTime(), size: info.Size()})
	}

	sort.Slice(kept, func(i, j int) bool {
		return kept[i].modTime.After(kept[j].modTime)
	})

	// current file is not in kept, so retain maxLogFiles-1 older copies
	limit := maxLogFiles - 1
	if limit < 0 {
		limit = 0
	}
	keepCount := limit
	if keepCount > len(kept) {
		keepCount = len(kept)
	}
	for _, extra := range kept[keepCount:] {
		_ = os.Remove(extra.path)
	}
	return nil
}

func reportLogCleanup(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("Warning: Failed to inspect logs directory: %v", err)
		return
	}

	var total int64
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !isAppLogFile(entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		count++
		total += info.Size()
	}

	log.Printf("Log retention: %d file(s), %.2f MB total (max %d MB/file, keep %d files, %d days)",
		count,
		float64(total)/1024/1024,
		maxLogFileSize/1024/1024,
		maxLogFiles,
		logRetentionDays)
}

var _ io.Writer = (*rotatingFileWriter)(nil)
