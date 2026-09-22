package main

import (
	"testing"
	"time"
)

func TestManualOverrideBlocksThorConditionChange(t *testing.T) {
	tr := &LightningTrigger{
		LastCondition: "Caution",
		failover:      defaultLightningFailoverPolicy(),
		feedHealth:    map[string]*FeedHealth{},
	}
	feed := &LightningFeedConfig{ID: "primary", Enabled: true}

	if _, err := tr.ApplyManualOverride("redalert", "test"); err != nil {
		t.Fatal(err)
	}
	if !tr.isManualOverrideActive() || !tr.redAlertActive {
		t.Fatal("expected override lock + Red Alert active")
	}

	tr.applyThorDrivenCondition(feed, "AllClear")
	if !tr.redAlertActive {
		t.Fatal("Thor AllClear must not unlock while override lock is active")
	}
	if tr.LastCondition != "RedAlert" {
		t.Fatalf("LastCondition should stay RedAlert, got %s", tr.LastCondition)
	}

	if _, err := tr.ApplyManualOverride("clear", "test"); err != nil {
		t.Fatal(err)
	}
	if tr.isManualOverrideActive() {
		t.Fatal("override lock should be released")
	}
	if !tr.redAlertActive {
		t.Fatal("releasing override must not clear Red Alert by itself")
	}

	// After release, Thor-driven AllClear may unlock (normal gates).
	tr.LastCondition = "RedAlert"
	tr.LastConditionTime = time.Now()
	tr.processConditionChange(feed, "AllClear")
	if tr.redAlertActive {
		t.Fatal("after release, authorized AllClear should unlock")
	}
}
