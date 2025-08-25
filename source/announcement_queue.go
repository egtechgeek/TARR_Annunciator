package main

import (
	"container/heap"
	"fmt"
	"log"
	"sync"
	"time"
)

// AnnouncementPriority defines the priority levels for announcements
type AnnouncementPriority int

const (
	PriorityLow    AnnouncementPriority = 1
	PriorityNormal AnnouncementPriority = 2
	PriorityHigh   AnnouncementPriority = 3
	PriorityCritical AnnouncementPriority = 4
	PriorityEmergency AnnouncementPriority = 5
)

// AnnouncementType defines the type of announcement
type AnnouncementType string

const (
	TypeStation     AnnouncementType = "station"
	TypeSafety      AnnouncementType = "safety"
	TypePromo       AnnouncementType = "promo"
	TypeEmergency   AnnouncementType = "emergency"
	TypeMaintenance AnnouncementType = "maintenance"
)

// AnnouncementStatus defines the current status of an announcement
type AnnouncementStatus string

const (
	StatusQueued  AnnouncementStatus = "queued"
	StatusPlaying AnnouncementStatus = "playing"
	StatusCompleted AnnouncementStatus = "completed"
	StatusCancelled AnnouncementStatus = "cancelled"
	StatusFailed    AnnouncementStatus = "failed"
)

// Announcement represents a single announcement in the queue
type Announcement struct {
	ID          string                 `json:"id"`
	Type        AnnouncementType       `json:"type"`
	Priority    AnnouncementPriority   `json:"priority"`
	Status      AnnouncementStatus     `json:"status"`
	CreatedAt   time.Time             `json:"created_at"`
	ScheduledAt time.Time             `json:"scheduled_at,omitempty"`
	StartedAt   *time.Time            `json:"started_at,omitempty"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"`
	AudioFiles  []string              `json:"audio_files"`
	Duration    time.Duration         `json:"duration,omitempty"`
	Error       string                `json:"error,omitempty"`
	
	// Internal fields for queue management
	index int // Index in the heap
}

// AnnouncementQueue is a priority queue for managing announcements
type AnnouncementQueue []*Announcement

// Implement heap.Interface for priority queue
func (aq AnnouncementQueue) Len() int { return len(aq) }

func (aq AnnouncementQueue) Less(i, j int) bool {
	// Higher priority comes first
	if aq[i].Priority != aq[j].Priority {
		return aq[i].Priority > aq[j].Priority
	}
	// If same priority, earlier scheduled time comes first
	return aq[i].ScheduledAt.Before(aq[j].ScheduledAt)
}

func (aq AnnouncementQueue) Swap(i, j int) {
	aq[i], aq[j] = aq[j], aq[i]
	aq[i].index = i
	aq[j].index = j
}

func (aq *AnnouncementQueue) Push(x interface{}) {
	n := len(*aq)
	item := x.(*Announcement)
	item.index = n
	*aq = append(*aq, item)
}

func (aq *AnnouncementQueue) Pop() interface{} {
	old := *aq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*aq = old[0 : n-1]
	return item
}

// AnnouncementManager manages the announcement queue and playback
type AnnouncementManager struct {
	queue           *AnnouncementQueue
	history         []*Announcement
	mutex           sync.RWMutex
	playing         *Announcement
	stopChan        chan bool
	isRunning       bool
	maxHistory      int
	nextID          int64
}

// Global announcement manager instance
var announcementManager *AnnouncementManager

// Global audio mutex to prevent any audio overlap
var globalAudioMutex sync.Mutex

// InitializeAnnouncementManager initializes the global announcement manager
func InitializeAnnouncementManager() {
	announcementManager = &AnnouncementManager{
		queue:      &AnnouncementQueue{},
		history:    make([]*Announcement, 0),
		stopChan:   make(chan bool),
		maxHistory: 100, // Keep last 100 announcements in history
		nextID:     1,
	}
	heap.Init(announcementManager.queue)
	
	// Start the announcement processor
	go announcementManager.processQueue()
	log.Printf("Announcement manager initialized with queuing system")
}

// generateID generates a unique ID for announcements
func (am *AnnouncementManager) generateID() string {
	am.nextID++
	return fmt.Sprintf("ann_%d_%d", time.Now().Unix(), am.nextID)
}

// QueueAnnouncement adds a new announcement to the queue
func (am *AnnouncementManager) QueueAnnouncement(announcementType AnnouncementType, priority AnnouncementPriority, parameters map[string]interface{}, scheduledAt time.Time) (*Announcement, error) {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	announcement := &Announcement{
		ID:          am.generateID(),
		Type:        announcementType,
		Priority:    priority,
		Status:      StatusQueued,
		CreatedAt:   time.Now(),
		ScheduledAt: scheduledAt,
		Parameters:  parameters,
	}
	
	// Build audio file paths based on announcement type
	var err error
	announcement.AudioFiles, err = am.buildAudioSequence(announcementType, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to build audio sequence: %v", err)
	}
	
	// Add to queue
	heap.Push(announcementManager.queue, announcement)
	
	log.Printf("Queued announcement: ID=%s, Type=%s, Priority=%d, Scheduled=%s", 
		announcement.ID, announcement.Type, announcement.Priority, announcement.ScheduledAt.Format(time.RFC3339))
	
	return announcement, nil
}

// buildAudioSequence builds the sequence of audio files for an announcement
func (am *AnnouncementManager) buildAudioSequence(announcementType AnnouncementType, parameters map[string]interface{}) ([]string, error) {
	var audioFiles []string
	
	switch announcementType {
	case TypeStation:
		// Station announcement sequence: chime + train + direction + destination + track
		audioFiles = []string{
			fmt.Sprintf("%s/chime.mp3", app.Config.MP3Dir),
			fmt.Sprintf("%s/train/%s.mp3", app.Config.MP3Dir, parameters["train_number"]),
			fmt.Sprintf("%s/direction/%s.mp3", app.Config.MP3Dir, parameters["direction"]),
			fmt.Sprintf("%s/destination/%s.mp3", app.Config.MP3Dir, parameters["destination"]),
			fmt.Sprintf("%s/track/%s.mp3", app.Config.MP3Dir, parameters["track_number"]),
		}
		
	case TypeSafety:
		// Safety announcement
		language := parameters["language"].(string)
		audioFiles = []string{
			fmt.Sprintf("%s/safety/safety_%s.mp3", app.Config.MP3Dir, language),
		}
		
	case TypePromo:
		// Promotional announcement
		file := parameters["file"].(string)
		audioFiles = []string{
			fmt.Sprintf("%s/promo/%s.mp3", app.Config.MP3Dir, file),
		}
		
	case TypeEmergency:
		// Emergency announcement (highest priority, audio files only)
		if emergencyFile, ok := parameters["file"].(string); ok {
			audioFiles = []string{
				fmt.Sprintf("%s/emergency/%s.mp3", app.Config.MP3Dir, emergencyFile),
			}
		} else {
			return nil, fmt.Errorf("emergency announcement requires 'file' parameter")
		}
		
	default:
		return nil, fmt.Errorf("unsupported announcement type: %s", announcementType)
	}
	
	return audioFiles, nil
}

// processQueue continuously processes the announcement queue
func (am *AnnouncementManager) processQueue() {
	am.isRunning = true
	ticker := time.NewTicker(100 * time.Millisecond) // Check queue every 100ms
	defer ticker.Stop()
	
	for am.isRunning {
		select {
		case <-am.stopChan:
			am.isRunning = false
			return
			
		case <-ticker.C:
			am.processNextAnnouncement()
		}
	}
}

// processNextAnnouncement processes the next announcement in the queue
func (am *AnnouncementManager) processNextAnnouncement() {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	// If currently playing, don't start another
	if am.playing != nil {
		return
	}
	
	// Check if there's anything in the queue
	if am.queue.Len() == 0 {
		return
	}
	
	// Get the next announcement (highest priority, earliest scheduled time)
	next := heap.Pop(am.queue).(*Announcement)
	
	// Check if it's time to play this announcement
	if next.ScheduledAt.After(time.Now()) {
		// Not time yet, put it back in the queue
		heap.Push(am.queue, next)
		return
	}
	
	// Start playing the announcement
	am.playing = next
	next.Status = StatusPlaying
	now := time.Now()
	next.StartedAt = &now
	
	log.Printf("Starting announcement: ID=%s, Type=%s, Priority=%d", 
		next.ID, next.Type, next.Priority)
	
	// Play the announcement in a separate goroutine
	go am.playAnnouncement(next)
}

// playAnnouncement plays a single announcement
func (am *AnnouncementManager) playAnnouncement(announcement *Announcement) {
	startTime := time.Now()
	
	// Play the audio sequence
	err := am.playAnnouncementAudio(announcement.AudioFiles)
	
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	// Update announcement status
	now := time.Now()
	announcement.CompletedAt = &now
	announcement.Duration = now.Sub(startTime)
	
	if err != nil {
		announcement.Status = StatusFailed
		announcement.Error = err.Error()
		log.Printf("Failed to play announcement: ID=%s, Error=%v", announcement.ID, err)
	} else {
		announcement.Status = StatusCompleted
		log.Printf("Completed announcement: ID=%s, Duration=%s", 
			announcement.ID, announcement.Duration.String())
	}
	
	// Move to history
	am.addToHistory(announcement)
	
	// Clear currently playing
	am.playing = nil
}

// playAnnouncementAudio plays the audio files for an announcement with proper synchronization
func (am *AnnouncementManager) playAnnouncementAudio(audioFiles []string) error {
	// Lock the global audio mutex to prevent any audio overlap
	globalAudioMutex.Lock()
	defer globalAudioMutex.Unlock()
	
	log.Printf("🔒 Audio mutex locked - starting announcement playback")
	
	for _, filePath := range audioFiles {
		if !fileExists(filePath) {
			log.Printf("Missing audio file: %s", filePath)
			continue
		}
		
		if err := playAudio(filePath); err != nil {
			log.Printf("🔓 Audio mutex unlocked due to error")
			return fmt.Errorf("error playing %s: %v", filePath, err)
		}
		
		// Small gap between audio files
		time.Sleep(300 * time.Millisecond)
	}
	
	log.Printf("🔓 Audio mutex unlocked - announcement playback complete")
	return nil
}

// addToHistory adds an announcement to the history and manages history size
func (am *AnnouncementManager) addToHistory(announcement *Announcement) {
	am.history = append(am.history, announcement)
	
	// Trim history if it exceeds maximum
	if len(am.history) > am.maxHistory {
		am.history = am.history[len(am.history)-am.maxHistory:]
	}
}

// GetQueueStatus returns the current status of the announcement queue
func (am *AnnouncementManager) GetQueueStatus() map[string]interface{} {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	
	queueItems := make([]*Announcement, len(*am.queue))
	copy(queueItems, *am.queue)
	
	return map[string]interface{}{
		"queue_length":    len(*am.queue),
		"currently_playing": am.playing,
		"queue_items":     queueItems,
		"history_count":   len(am.history),
		"is_running":      am.isRunning,
	}
}

// GetHistory returns the announcement history
func (am *AnnouncementManager) GetHistory(limit int) []*Announcement {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	
	if limit <= 0 || limit > len(am.history) {
		limit = len(am.history)
	}
	
	// Return the most recent items
	start := len(am.history) - limit
	if start < 0 {
		start = 0
	}
	
	result := make([]*Announcement, limit)
	copy(result, am.history[start:])
	
	return result
}

// CancelAnnouncement cancels a queued announcement
func (am *AnnouncementManager) CancelAnnouncement(id string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	// Find the announcement in the queue
	for i, announcement := range *am.queue {
		if announcement.ID == id {
			if announcement.Status == StatusQueued {
				announcement.Status = StatusCancelled
				now := time.Now()
				announcement.CompletedAt = &now
				
				// Remove from queue
				heap.Remove(am.queue, i)
				
				// Add to history
				am.addToHistory(announcement)
				
				log.Printf("Cancelled announcement: ID=%s", id)
				return nil
			} else {
				return fmt.Errorf("cannot cancel announcement with status: %s", announcement.Status)
			}
		}
	}
	
	return fmt.Errorf("announcement not found: %s", id)
}

// Stop stops the announcement manager
func (am *AnnouncementManager) Stop() {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	if am.isRunning {
		am.isRunning = false
		am.stopChan <- true
		log.Printf("Announcement manager stopped")
	}
}

// Helper function to get priority from string
func ParsePriority(priorityStr string) AnnouncementPriority {
	switch priorityStr {
	case "low":
		return PriorityLow
	case "normal":
		return PriorityNormal
	case "high":
		return PriorityHigh
	case "critical":
		return PriorityCritical
	case "emergency":
		return PriorityEmergency
	default:
		return PriorityNormal
	}
}

// Helper function to convert priority to string
func (p AnnouncementPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	case PriorityEmergency:
		return "emergency"
	default:
		return "normal"
	}
}