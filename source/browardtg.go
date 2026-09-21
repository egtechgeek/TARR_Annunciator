package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// BrowardTGSensor is one known Broward Thor Guard XML endpoint from browardtg.json.
type BrowardTGSensor struct {
	ID          string `json:"id"`
	PathPrefix  string `json:"path_prefix"`
	DisplayName string `json:"display_name"`
	URL         string `json:"url"`
}

// BrowardTGCatalog is the on-disk sensor catalog (not operator-secret).
type BrowardTGCatalog struct {
	SchemaVersion int               `json:"schema_version"`
	Region        string            `json:"region"`
	Host          string            `json:"host"`
	Source        string            `json:"source"`
	GeneratedFor  string            `json:"generated_for"`
	Sensors       []BrowardTGSensor `json:"sensors"`
}

var (
	browardTGCatalog   *BrowardTGCatalog
	browardTGCatalogMu sync.RWMutex
)

func browardtgPath() string {
	if app != nil && app.Config != nil && app.Config.JSONDir != "" {
		return filepath.Join(app.Config.JSONDir, "browardtg.json")
	}
	return filepath.Join("json", "browardtg.json")
}

func loadBrowardTGCatalog() error {
	path := browardtgPath()
	data, err := os.ReadFile(path)
	if err != nil {
		browardTGCatalogMu.Lock()
		browardTGCatalog = &BrowardTGCatalog{Sensors: nil}
		browardTGCatalogMu.Unlock()
		if os.IsNotExist(err) {
			log.Printf("Warning: browardtg.json not found at %s — Admin catalog dropdown will be empty", path)
			return nil
		}
		log.Printf("Warning: failed to read browardtg.json: %v", err)
		return err
	}
	var cat BrowardTGCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		log.Printf("Warning: failed to parse browardtg.json: %v", err)
		browardTGCatalogMu.Lock()
		browardTGCatalog = &BrowardTGCatalog{Sensors: nil}
		browardTGCatalogMu.Unlock()
		return err
	}
	browardTGCatalogMu.Lock()
	browardTGCatalog = &cat
	browardTGCatalogMu.Unlock()
	log.Printf("✓ Loaded Broward Thor Guard catalog (%d sensors)", len(cat.Sensors))
	return nil
}

func getBrowardTGCatalog() BrowardTGCatalog {
	browardTGCatalogMu.RLock()
	defer browardTGCatalogMu.RUnlock()
	if browardTGCatalog == nil {
		return BrowardTGCatalog{Sensors: []BrowardTGSensor{}}
	}
	out := *browardTGCatalog
	out.Sensors = append([]BrowardTGSensor(nil), browardTGCatalog.Sensors...)
	return out
}

func getBrowardTGSensors() []BrowardTGSensor {
	return getBrowardTGCatalog().Sensors
}

func findBrowardTGSensorByURL(rawURL string) *BrowardTGSensor {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return nil
	}
	sensors := getBrowardTGSensors()
	for i := range sensors {
		if strings.EqualFold(strings.TrimSpace(sensors[i].URL), u) {
			cp := sensors[i]
			return &cp
		}
	}
	return nil
}

func findBrowardTGSensorByID(id string) *BrowardTGSensor {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	sensors := getBrowardTGSensors()
	for i := range sensors {
		if strings.EqualFold(sensors[i].ID, id) {
			cp := sensors[i]
			return &cp
		}
	}
	return nil
}
