package main

import (
	"strings"
	"testing"
	"time"
)

func TestFetchOneFeedResolvingUnknown_PassesThroughNonUnknown(t *testing.T) {
	// Unit-level: soft-record and alert classification helpers used by Unknown path.
	tr := &LightningTrigger{feedHealth: map[string]*FeedHealth{}}
	tr.recordUnknownSoft("primary", feedFetchResult{
		ok: true, alert: "Unknown", displayname: "Tradewinds Park", uniqueid: "FL0115",
	})
	h := tr.healthFor("primary")
	if h.LastAlert != "Unknown" {
		t.Fatalf("expected LastAlert Unknown, got %q", h.LastAlert)
	}
	if h.ConsecutiveFailures != 0 {
		t.Fatalf("Unknown soft must not increment failures, got %d", h.ConsecutiveFailures)
	}
	if h.ConsecutiveSuccesses != 0 {
		t.Fatalf("Unknown soft must not increment successes, got %d", h.ConsecutiveSuccesses)
	}
	if !strings.Contains(h.LastError, "not counted as failure") {
		t.Fatalf("expected soft error note, got %q", h.LastError)
	}
	if time.Since(h.LastErrorAt) > time.Minute {
		t.Fatal("LastErrorAt should be recent")
	}
}
