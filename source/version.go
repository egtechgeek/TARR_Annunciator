package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AppVersion is the product release version embedded in this binary.
// Override at build time: -ldflags "-X main.AppVersion=1.1.2"
//
// Deployed Pi units before in-app updater work are treated as v1.0.0
// (see BaselineInstalledVersion). This codebase ships as v1.1.2.
var AppVersion = "1.1.2"

// BaselineInstalledVersion is assumed when install_version.json is missing
// (current live Pi before in-app updates existed).
const BaselineInstalledVersion = "1.0.0"

const installVersionFile = "install_version.json"

type installVersionRecord struct {
	AppVersion  string `json:"app_version"`
	InstalledAt string `json:"installed_at"`
	Source      string `json:"source,omitempty"`
}

func installVersionPath() string {
	if app != nil && app.Config != nil && app.Config.JSONDir != "" {
		return filepath.Join(app.Config.JSONDir, installVersionFile)
	}
	return filepath.Join("json", installVersionFile)
}

func getInstalledVersion() string {
	path := installVersionPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return BaselineInstalledVersion
	}
	var rec installVersionRecord
	if err := json.Unmarshal(data, &rec); err != nil || strings.TrimSpace(rec.AppVersion) == "" {
		return BaselineInstalledVersion
	}
	return strings.TrimPrefix(strings.TrimSpace(rec.AppVersion), "v")
}

func writeInstallVersion(version, source string) error {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" {
		version = AppVersion
	}
	rec := installVersionRecord{
		AppVersion:  version,
		InstalledAt: time.Now().Format(time.RFC3339),
		Source:      source,
	}
	data, err := json.MarshalIndent(rec, "", "    ")
	if err != nil {
		return err
	}
	path := installVersionPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// compareSemver returns -1 if a<b, 0 if equal, 1 if a>b. Non-semver falls back to string compare.
func compareSemver(a, b string) int {
	a = normalizeVersion(a)
	b = normalizeVersion(b)
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(ap) {
			fmtSscanfInt(ap[i], &ai)
		}
		if i < len(bp) {
			fmtSscanfInt(bp[i], &bi)
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}

func fmtSscanfInt(s string, out *int) {
	s = strings.TrimSpace(s)
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	*out = n
}

func versionStatusMap() map[string]interface{} {
	installed := getInstalledVersion()
	return map[string]interface{}{
		"app_version":       normalizeVersion(AppVersion),
		"installed_version": installed,
		"baseline_version":  BaselineInstalledVersion,
	}
}
