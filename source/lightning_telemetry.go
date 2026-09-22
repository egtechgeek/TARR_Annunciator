package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

const telemetryHistoryMax = 16

// parseThorFloat parses a Thor XML numeric tag; ok=false if empty/unparseable.
func parseThorFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func extractThorTelemetry(xmlString string) (lhl, di, ad float64, ok bool) {
	lhlV, lhlOK := parseThorFloat(extractXMLTag(xmlString, "lhl"))
	diV, diOK := parseThorFloat(extractXMLTag(xmlString, "di"))
	adV, adOK := parseThorFloat(extractXMLTag(xmlString, "ad"))
	if !lhlOK || !diOK || !adOK {
		return lhlV, diV, adV, false
	}
	return lhlV, diV, adV, true
}

func (th TelemetryCollapseThresholds) normalized() TelemetryCollapseThresholds {
	def := defaultTelemetryCollapseThresholds()
	if th.HistorySamples <= 0 {
		th.HistorySamples = def.HistorySamples
	}
	if th.ElevatedLHL <= 0 {
		th.ElevatedLHL = def.ElevatedLHL
	}
	if th.ElevatedDI <= 0 {
		th.ElevatedDI = def.ElevatedDI
	}
	if th.ElevatedAD <= 0 {
		th.ElevatedAD = def.ElevatedAD
	}
	if th.FloorLHLMax <= 0 {
		th.FloorLHLMax = def.FloorLHLMax
	}
	return th
}

func (th TelemetryCollapseThresholds) isElevated(s TelemetrySample) bool {
	if !s.OK {
		return false
	}
	// All three must be elevated. LHL-only spikes (DI=0 AD=0) and AD countdown
	// flicker on clear days are not treated as storm energy for cliff detection.
	return s.LHL >= th.ElevatedLHL && s.DI >= th.ElevatedDI && s.AD >= th.ElevatedAD
}

func (th TelemetryCollapseThresholds) isFloor(s TelemetrySample) bool {
	if !s.OK {
		return false
	}
	return s.LHL <= th.FloorLHLMax && s.DI == 0 && s.AD == 0
}

// detectTelemetryCollapse returns true when the immediately previous sample had
// elevated LHL and DI and AD, and the current sample is an immediate floor cliff.
// Calm-day zeros, single-metric spikes, and gradual multi-poll declines do not trip.
// Alert tag is ignored (metric cliff alone).
func detectTelemetryCollapse(history []TelemetrySample, current TelemetrySample, th TelemetryCollapseThresholds) (bool, TelemetrySample) {
	th = th.normalized()
	var prev TelemetrySample
	if !current.OK || !th.isFloor(current) {
		return false, prev
	}
	if len(history) == 0 {
		return false, prev
	}
	prev = history[len(history)-1]
	if !th.isElevated(prev) {
		return false, prev
	}
	return true, prev
}

func appendTelemetrySample(history []TelemetrySample, sample TelemetrySample) []TelemetrySample {
	history = append(history, sample)
	if len(history) > telemetryHistoryMax {
		history = history[len(history)-telemetryHistoryMax:]
	}
	return history
}

func floatPtr(v float64) *float64 {
	x := v
	return &x
}

// telemetryActivityRestored is the sticky-collapse clear gate: sensor must show
// DI > 0 or AD > 0 (float DI decimals supported) before the feed may drive conditions again.
func telemetryActivityRestored(di, ad float64) bool {
	return di > 0 || ad > 0
}

// allClearVoteBallotAccepted decides whether a failover vote probe counts as AllClear for unlock.
// Sticky collapse / other fetch failures never count. AllClear with floor metrics (DI=0 and AD=0)
// also never counts — regional Thor fake-AllClear must not unlock via failover_vote.
func allClearVoteBallotAccepted(res feedFetchResult) (accepted bool, detail string) {
	if !res.ok {
		class := res.failureClass
		if class == "" {
			class = "error"
		}
		return false, fmt.Sprintf("%s: %s", class, res.errMsg)
	}
	alert := strings.TrimSpace(res.alert)
	if !strings.EqualFold(alert, "allclear") {
		return false, alert
	}
	if !res.metricsOK || !telemetryActivityRestored(res.di, res.ad) {
		return false, fmt.Sprintf("AllClear_untrusted_floor (lhl=%.2f di=%.2f ad=%.2f)", res.lhl, res.di, res.ad)
	}
	return true, "AllClear"
}

// applyTelemetryToResult attaches LHL/DI/AD to a fetch result and, if a cliff is detected
// against existing feed history, reclassifies the result as telemetry_collapse failure.
// Collapse is sticky: the feed stays unreliable (no condition/trigger driving) until a later
// poll shows DI > 0 or AD > 0. Fetches continue either way. Layer A is always the !ok path
// (caller must not processConditionChange). Layer B uses triggerEnabled separately.
func (t *LightningTrigger) applyTelemetryToResult(feedID string, res feedFetchResult) feedFetchResult {
	if res.xmlString == "" {
		return res
	}
	lhl, di, ad, metricsOK := extractThorTelemetry(res.xmlString)
	sample := TelemetrySample{At: time.Now(), LHL: lhl, DI: di, AD: ad, OK: metricsOK}
	res.lhl, res.di, res.ad = lhl, di, ad
	res.metricsOK = metricsOK

	th := defaultTelemetryCollapseThresholds()
	t.mu.Lock()
	th = t.failover.TelemetryCollapse.normalized()
	h := t.healthForLocked(feedID)
	wasSticky := h.TelemetryCollapse
	prior := append([]TelemetrySample(nil), h.TelemetryHistory...)
	collapsed := false
	var prev TelemetrySample
	if res.ok && metricsOK {
		collapsed, prev = detectTelemetryCollapse(prior, sample, th)
	}
	h.TelemetryHistory = appendTelemetrySample(h.TelemetryHistory, sample)
	if metricsOK {
		h.LastLHL = floatPtr(lhl)
		h.LastDI = floatPtr(di)
		h.LastAD = floatPtr(ad)
	}
	if collapsed {
		h.TelemetryCollapse = true
		res.ok = false
		res.failureClass = "telemetry_collapse"
		res.errMsg = fmt.Sprintf(
			"LHL/DI/AD cliff to floor (now lhl=%.2f di=%.2f ad=%.2f; prev lhl=%.2f di=%.2f ad=%.2f)",
			lhl, di, ad, prev.LHL, prev.DI, prev.AD,
		)
		res.alert = "Unknown"
		log.Printf("Lightning feed %s: telemetry_collapse — %s", feedID, res.errMsg)
	} else if wasSticky {
		if metricsOK && telemetryActivityRestored(di, ad) {
			h.TelemetryCollapse = false
			log.Printf("Lightning feed %s: telemetry_collapse cleared — DI/AD activity restored (lhl=%.2f di=%.2f ad=%.2f)", feedID, lhl, di, ad)
		} else {
			h.TelemetryCollapse = true
			res.ok = false
			res.failureClass = "telemetry_collapse"
			res.errMsg = fmt.Sprintf(
				"telemetry_collapse hold (awaiting DI>0 or AD>0; now lhl=%.2f di=%.2f ad=%.2f)",
				lhl, di, ad,
			)
			res.alert = "Unknown"
			log.Printf("Lightning feed %s: telemetry_collapse — %s", feedID, res.errMsg)
		}
	}
	t.mu.Unlock()
	return res
}

func (t *LightningTrigger) healthForLocked(feedID string) *FeedHealth {
	if t.feedHealth == nil {
		t.feedHealth = map[string]*FeedHealth{}
	}
	h, ok := t.feedHealth[feedID]
	if !ok || h == nil {
		h = &FeedHealth{}
		t.feedHealth[feedID] = h
	}
	return h
}

// feedHasTelemetryCollapse reports whether the feed is in sticky telemetry_collapse
// (Layer A unlock denial — always enforced until DI/AD activity restores).
func (t *LightningTrigger) feedHasTelemetryCollapse(feedID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	h := t.healthForLocked(feedID)
	return h.TelemetryCollapse
}
