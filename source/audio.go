package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

// Audio playback functions
func playAudio(filePath string) error {
	if !app.AudioEnabled {
		log.Printf("Audio not available - would play: %s", filePath)
		return fmt.Errorf("audio not available")
	}

	if !fileExists(filePath) {
		log.Printf("Audio file not found: %s", filePath)
		return fmt.Errorf("audio file not found: %s", filePath)
	}

	log.Printf("Playing audio: %s (Volume: %d%%)", filePath, int(app.Config.CurrentVolume*100))

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open audio file: %v", err)
	}
	defer file.Close()

	// Decode the MP3
	streamer, format, err := mp3.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode MP3: %v", err)
	}
	defer streamer.Close()

	// Resample if necessary
	resampled := beep.Resample(4, format.SampleRate, beep.SampleRate(44100), streamer)

	// Apply volume
	volume := &effects.Volume{
		Streamer: resampled,
		Base:     2,
		Volume:   0, // Will be set below
		Silent:   false,
	}
	
	// Convert linear volume (0.0-1.0) to logarithmic scale
	if app.Config.CurrentVolume <= 0.0 {
		volume.Silent = true
	} else {
		// Convert to decibels: 20 * log10(volume)
		// But since beep uses base 2, we need different calculation
		volume.Volume = (app.Config.CurrentVolume - 1.0) * 5 // Approximate conversion
	}

	// Create a done channel to wait for playback completion
	done := make(chan bool)
	speaker.Play(beep.Seq(volume, beep.Callback(func() {
		done <- true
	})))

	// Wait for playback to complete
	<-done

	return nil
}

func playAudioSequence(filePaths []string) {
	// Note: This function should only be called when already holding the globalAudioMutex
	// The mutex locking is handled by the caller to prevent deadlocks
	for _, filePath := range filePaths {
		if fileExists(filePath) {
			log.Printf("Playing: %s", filepath.Base(filePath))
			if err := playAudio(filePath); err != nil {
				log.Printf("Error playing %s: %v", filePath, err)
			}
			time.Sleep(300 * time.Millisecond) // Small gap between announcements
		} else {
			log.Printf("Missing audio file: %s", filePath)
		}
	}
}

func playStationAnnouncement(trainNumber, direction, destination, trackNumber string) {
	// DEPRECATED: This function now uses the announcement queue system
	log.Printf("⚠️  DEPRECATED: playStationAnnouncement called - routing through queue system")
	
	// Route through queue system with normal priority
	parameters := map[string]interface{}{
		"train_number": trainNumber,
		"direction":    direction,
		"destination":  destination,
		"track_number": trackNumber,
	}
	
	if announcementManager != nil {
		announcementManager.QueueAnnouncement(TypeStation, PriorityNormal, parameters, time.Now())
	} else {
		log.Printf("⚠️  Announcement manager not initialized - falling back to direct audio")
		globalAudioMutex.Lock()
		defer globalAudioMutex.Unlock()
		
		audioSequence := []string{
			filepath.Join(app.Config.MP3Dir, "chime.mp3"),
			filepath.Join(app.Config.MP3Dir, "train", trainNumber+".mp3"),
			filepath.Join(app.Config.MP3Dir, "direction", direction+".mp3"),
			filepath.Join(app.Config.MP3Dir, "destination", destination+".mp3"),
			filepath.Join(app.Config.MP3Dir, "track", trackNumber+".mp3"),
		}
		playAudioSequence(audioSequence)
	}
}

func playPromo(file string) {
	// DEPRECATED: This function now uses the announcement queue system
	log.Printf("⚠️  DEPRECATED: playPromo called - routing through queue system")
	
	// Route through queue system with low priority
	parameters := map[string]interface{}{
		"file": file,
	}
	
	if announcementManager != nil {
		announcementManager.QueueAnnouncement(TypePromo, PriorityLow, parameters, time.Now())
	} else {
		log.Printf("⚠️  Announcement manager not initialized - falling back to direct audio")
		globalAudioMutex.Lock()
		defer globalAudioMutex.Unlock()
		
		promoFile := filepath.Join(app.Config.MP3Dir, "promo", file+".mp3")
		if err := playAudio(promoFile); err != nil {
			log.Printf("Error playing promo: %v", err)
		}
	}
}

func playSafety(language string) {
	// DEPRECATED: This function now uses the announcement queue system
	log.Printf("⚠️  DEPRECATED: playSafety called - routing through queue system")
	
	// Route through queue system with high priority (safety is important)
	parameters := map[string]interface{}{
		"language": language,
	}
	
	if announcementManager != nil {
		announcementManager.QueueAnnouncement(TypeSafety, PriorityHigh, parameters, time.Now())
	} else {
		log.Printf("⚠️  Announcement manager not initialized - falling back to direct audio")
		globalAudioMutex.Lock()
		defer globalAudioMutex.Unlock()
		
		safetyFile := filepath.Join(app.Config.MP3Dir, "safety", "safety_"+language+".mp3")
		if err := playAudio(safetyFile); err != nil {
			log.Printf("Error playing safety announcement: %v", err)
		}
	}
}