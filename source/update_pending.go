package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const updatePendingDirName = "update_pending"
const updatePendingMetaFile = "meta.json"

// updatePendingMeta is written by the applying binary before swap; consumed on next startup.
type updatePendingMeta struct {
	FromVersion   string `json:"from_version"`
	TargetVersion string `json:"target_version"`
	SeedsRelpath  string `json:"seeds_relpath"` // relative to update_pending/, usually "json"
	TagName       string `json:"tag_name,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
}

func updatePendingDir() string {
	if app != nil && app.Config != nil && app.Config.BaseDir != "" {
		return filepath.Join(app.Config.BaseDir, updatePendingDirName)
	}
	return updatePendingDirName
}

func updatePendingMetaPath() string {
	return filepath.Join(updatePendingDir(), updatePendingMetaFile)
}

func writeUpdatePendingMeta(meta updatePendingMeta) error {
	dir := updatePendingDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(updatePendingMetaPath(), append(data, '\n'), 0644)
}

func readUpdatePendingMeta() (*updatePendingMeta, error) {
	data, err := os.ReadFile(updatePendingMetaPath())
	if err != nil {
		return nil, err
	}
	var meta updatePendingMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func clearUpdatePending() {
	dir := updatePendingDir()
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("Warning: could not remove update_pending: %v", err)
	}
}

// processUpdatePendingIfAny runs package-seed + schema migrations after a binary swap
// when the previous apply deferred install_version finalize.
func processUpdatePendingIfAny() error {
	meta, err := readUpdatePendingMeta()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read update_pending meta: %w", err)
	}
	from := normalizeVersion(meta.FromVersion)
	if from == "" {
		from = getInstalledVersion()
	}
	target := normalizeVersion(meta.TargetVersion)
	if target == "" {
		target = normalizeVersion(AppVersion)
	}
	seedsRel := strings.TrimSpace(meta.SeedsRelpath)
	if seedsRel == "" {
		seedsRel = "json"
	}
	seedsDir := filepath.Join(updatePendingDir(), seedsRel)
	log.Printf("Update pending: applying migrations %s → %s (seeds=%s)", from, target, seedsDir)

	if err := migrateFromPackageSeeds(seedsDir); err != nil {
		return fmt.Errorf("pending package seeds: %w", err)
	}
	if err := runSchemaMigrations(from); err != nil {
		return fmt.Errorf("pending schema migrations: %w", err)
	}
	src := "github-release"
	if meta.TagName != "" {
		src = "github-release:" + meta.TagName
	}
	if err := writeInstallVersion(target, src); err != nil {
		return fmt.Errorf("finalize install_version: %w", err)
	}
	clearUpdatePending()
	log.Printf("Update pending: finalized install_version=%s", target)
	return nil
}

// stageUpdatePendingJSON copies package json/ into update_pending/json for post-restart migrations.
func stageUpdatePendingJSON(packageJSONDir string) error {
	dest := filepath.Join(updatePendingDir(), "json")
	_ = os.RemoveAll(dest)
	if err := copyDir(packageJSONDir, dest); err != nil {
		return err
	}
	return nil
}

// nowRFC3339UTC is a tiny helper for pending meta timestamps.
func nowRFC3339UTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
