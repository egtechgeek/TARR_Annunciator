package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	consoleBufferMaxLines = 2500
	consoleSubscriberBuf  = 256
)

// consoleHub tees application log output into a ring buffer and live SSE subscribers.
type consoleHub struct {
	mu       sync.RWMutex
	lines    []string
	maxLines int
	partial  string
	subs     map[chan string]struct{}
}

var appConsoleHub *consoleHub

func newConsoleHub(maxLines int) *consoleHub {
	if maxLines < 100 {
		maxLines = 100
	}
	return &consoleHub{
		lines:    make([]string, 0, maxLines),
		maxLines: maxLines,
		subs:     make(map[chan string]struct{}),
	}
}

func (h *consoleHub) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	h.partial += string(p)
	for {
		idx := strings.IndexByte(h.partial, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimRight(h.partial[:idx], "\r")
		h.partial = h.partial[idx+1:]
		h.appendLineLocked(line)
	}
	return len(p), nil
}

func (h *consoleHub) appendLineLocked(line string) {
	h.lines = append(h.lines, line)
	if len(h.lines) > h.maxLines {
		h.lines = h.lines[len(h.lines)-h.maxLines:]
	}
	for ch := range h.subs {
		select {
		case ch <- line:
		default:
			// Slow subscriber — drop this line for them rather than block logging.
		}
	}
}

func (h *consoleHub) recent(limit int) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if limit <= 0 || limit > len(h.lines) {
		limit = len(h.lines)
	}
	out := make([]string, limit)
	copy(out, h.lines[len(h.lines)-limit:])
	return out
}

func (h *consoleHub) subscribe() chan string {
	ch := make(chan string, consoleSubscriberBuf)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *consoleHub) unsubscribe(ch chan string) {
	h.mu.Lock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *consoleHub) subscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}

// consoleSkipGinPath omits console endpoints from Gin access logs so the live
// view is not flooded by its own EventSource traffic.
func consoleSkipGinPath(path string) bool {
	return strings.HasPrefix(path, "/admin/console")
}

// ginLoggerWithConsoleSkip wraps gin.Logger so /admin/console* does not emit
// access lines (LoggerConfig.Skip requires gin newer than v1.9.1).
func ginLoggerWithConsoleSkip() gin.HandlerFunc {
	logger := gin.LoggerWithConfig(gin.LoggerConfig{
		Output: gin.DefaultWriter,
	})
	return func(c *gin.Context) {
		if consoleSkipGinPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		logger(c)
	}
}

func getConsoleRecentHandler(c *gin.Context) {
	if appConsoleHub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "console hub not initialized"})
		return
	}
	limit := 500
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > consoleBufferMaxLines {
		limit = consoleBufferMaxLines
	}
	c.JSON(http.StatusOK, gin.H{
		"lines":       appConsoleHub.recent(limit),
		"subscribers": appConsoleHub.subscriberCount(),
		"buffer_max":  consoleBufferMaxLines,
	})
}

func getConsoleStreamHandler(c *gin.Context) {
	if appConsoleHub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "console hub not initialized"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}

	for _, line := range appConsoleHub.recent(400) {
		writeConsoleSSE(c.Writer, line)
	}
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}

	ch := appConsoleHub.subscribe()
	defer appConsoleHub.unsubscribe(ch)

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	notify := c.Request.Context().Done()

	for {
		select {
		case <-notify:
			return
		case <-heartbeat.C:
			_, _ = io.WriteString(c.Writer, ": ping\n\n")
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush()
			}
		case line, ok := <-ch:
			if !ok {
				return
			}
			writeConsoleSSE(c.Writer, line)
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

func writeConsoleSSE(w http.ResponseWriter, line string) {
	payload, err := json.Marshal(line)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", payload)
}
