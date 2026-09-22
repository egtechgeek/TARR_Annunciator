package main

import (
	_ "embed"
	"log"
	"os"
	"path/filepath"
)

//go:embed embedded/browardtg.json
var embeddedBrowardTGJSON []byte

// ensureEmbeddedJSONSeeds writes critical catalog seeds when missing on disk.
// Safe to call every startup (including when from == to). Never overwrites existing files.
func ensureEmbeddedJSONSeeds() {
	if err := ensureEmbeddedBrowardTG(); err != nil {
		log.Printf("Warning: ensureEmbeddedBrowardTG: %v", err)
	}
}

func ensureEmbeddedBrowardTG() error {
	livePath := browardtgPath()
	if fileExists(livePath) {
		return nil
	}
	if len(embeddedBrowardTGJSON) == 0 {
		log.Printf("Warning: embedded browardtg.json is empty")
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(livePath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(livePath, embeddedBrowardTGJSON, 0644); err != nil {
		return err
	}
	log.Printf("Migration: installed browardtg.json from embedded seed (%d bytes)", len(embeddedBrowardTGJSON))
	_ = loadBrowardTGCatalog()
	return nil
}
