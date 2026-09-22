package main

import (
	"strings"
	"testing"
	"time"
)

func TestDetectTelemetryCollapse_Cliff(t *testing.T) {
	th := defaultTelemetryCollapseThresholds()
	history := []TelemetrySample{
		{At: time.Now().Add(-2 * time.Minute), LHL: 8, DI: 3.2, AD: 7, OK: true},
		{At: time.Now().Add(-1 * time.Minute), LHL: 7, DI: 3.0, AD: 5, OK: true},
	}
	current := TelemetrySample{At: time.Now(), LHL: 0, DI: 0, AD: 0, OK: true}
	ok, _ := detectTelemetryCollapse(history, current, th)
	if !ok {
		t.Fatal("expected cliff to trip telemetry_collapse")
	}
}

func TestDetectTelemetryCollapse_CalmDay(t *testing.T) {
	th := defaultTelemetryCollapseThresholds()
	history := []TelemetrySample{
		{At: time.Now().Add(-2 * time.Minute), LHL: 0, DI: 0, AD: 0, OK: true},
		{At: time.Now().Add(-1 * time.Minute), LHL: 1, DI: 0, AD: 0, OK: true},
	}
	current := TelemetrySample{At: time.Now(), LHL: 0, DI: 0, AD: 0, OK: true}
	ok, _ := detectTelemetryCollapse(history, current, th)
	if ok {
		t.Fatal("calm day floor must not trip")
	}
}

func TestDetectTelemetryCollapse_ADOnlyFlicker(t *testing.T) {
	th := defaultTelemetryCollapseThresholds()
	// Clear-day AD countdown/flicker must NOT trip when LHL/DI stay at floor.
	history := []TelemetrySample{
		{At: time.Now().Add(-1 * time.Minute), LHL: 0, DI: 0, AD: 3, OK: true},
	}
	current := TelemetrySample{At: time.Now(), LHL: 0, DI: 0, AD: 0, OK: true}
	ok, _ := detectTelemetryCollapse(history, current, th)
	if ok {
		t.Fatal("AD-only drop on clear day must not trip")
	}
}

func TestDetectTelemetryCollapse_LHLOnlyMustNotTrip(t *testing.T) {
	th := defaultTelemetryCollapseThresholds()
	// Observed field false positive: LHL=4 with DI=0 AD=0 then floor — not a full storm cliff.
	history := []TelemetrySample{
		{At: time.Now().Add(-1 * time.Minute), LHL: 4, DI: 0, AD: 0, OK: true},
	}
	current := TelemetrySample{At: time.Now(), LHL: 0, DI: 0, AD: 0, OK: true}
	ok, _ := detectTelemetryCollapse(history, current, th)
	if ok {
		t.Fatal("LHL-only elevated sample must not trip without DI and AD")
	}
}

func TestDetectTelemetryCollapse_MissingDIOrADMustNotTrip(t *testing.T) {
	th := defaultTelemetryCollapseThresholds()
	cases := []struct {
		name string
		prev TelemetrySample
	}{
		{"LHL+DI no AD", TelemetrySample{LHL: 6, DI: 3.0, AD: 0, OK: true}},
		{"LHL+AD no DI", TelemetrySample{LHL: 6, DI: 0, AD: 5, OK: true}},
		{"DI+AD no LHL", TelemetrySample{LHL: 1, DI: 3.0, AD: 5, OK: true}},
	}
	current := TelemetrySample{At: time.Now(), LHL: 0, DI: 0, AD: 0, OK: true}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, _ := detectTelemetryCollapse([]TelemetrySample{tc.prev}, current, th)
			if ok {
				t.Fatalf("%s must not trip — need all three elevated", tc.name)
			}
		})
	}
}

func TestDetectTelemetryCollapse_Gradual(t *testing.T) {
	th := defaultTelemetryCollapseThresholds()
	history := []TelemetrySample{
		{At: time.Now().Add(-3 * time.Minute), LHL: 8, DI: 3.0, AD: 5, OK: true},
		{At: time.Now().Add(-2 * time.Minute), LHL: 4, DI: 1.5, AD: 2, OK: true},
		{At: time.Now().Add(-1 * time.Minute), LHL: 2, DI: 0.5, AD: 0, OK: true},
	}
	current := TelemetrySample{At: time.Now(), LHL: 0, DI: 0, AD: 0, OK: true}
	ok, _ := detectTelemetryCollapse(history, current, th)
	if ok {
		t.Fatal("gradual decline to floor must not trip when previous sample was not elevated")
	}
}

func TestDetectTelemetryCollapse_OneStepPrevious(t *testing.T) {
	th := defaultTelemetryCollapseThresholds()
	history := []TelemetrySample{
		{At: time.Now().Add(-1 * time.Minute), LHL: 8, DI: 3.1, AD: 6, OK: true},
	}
	current := TelemetrySample{At: time.Now(), LHL: 0, DI: 0, AD: 0, OK: true}
	ok, _ := detectTelemetryCollapse(history, current, th)
	if !ok {
		t.Fatal("one-step elevated→floor must trip")
	}
}

func TestDetectTelemetryCollapse_ColdStart(t *testing.T) {
	th := defaultTelemetryCollapseThresholds()
	current := TelemetrySample{At: time.Now(), LHL: 0, DI: 0, AD: 0, OK: true}
	ok, _ := detectTelemetryCollapse(nil, current, th)
	if ok {
		t.Fatal("cold start must not trip")
	}
}

func TestDetectTelemetryCollapse_LHL1Floor(t *testing.T) {
	th := defaultTelemetryCollapseThresholds()
	history := []TelemetrySample{
		{At: time.Now().Add(-1 * time.Minute), LHL: 6, DI: 2.5, AD: 3, OK: true},
	}
	current := TelemetrySample{At: time.Now(), LHL: 1, DI: 0, AD: 0, OK: true}
	ok, _ := detectTelemetryCollapse(history, current, th)
	if !ok {
		t.Fatal("LHL=1 with DI=0 AD=0 after elevated should trip")
	}
}

func TestTelemetryActivityRestored(t *testing.T) {
	if telemetryActivityRestored(0, 0) {
		t.Fatal("floor must not restore")
	}
	if !telemetryActivityRestored(0.1, 0) {
		t.Fatal("DI decimal > 0 must restore")
	}
	if !telemetryActivityRestored(0, 1) {
		t.Fatal("AD > 0 must restore")
	}
	if !telemetryActivityRestored(2.3, 4) {
		t.Fatal("DI and AD activity must restore")
	}
}

func TestAllClearVoteBallotAccepted(t *testing.T) {
	ok, _ := allClearVoteBallotAccepted(feedFetchResult{
		ok: true, alert: "AllClear", metricsOK: true, lhl: 0, di: 0, ad: 0,
	})
	if ok {
		t.Fatal("floor AllClear must not count for failover_vote")
	}

	ok, _ = allClearVoteBallotAccepted(feedFetchResult{
		ok: true, alert: "AllClear", metricsOK: true, lhl: 0, di: 0.5, ad: 0,
	})
	if !ok {
		t.Fatal("AllClear with DI activity must count")
	}

	ok, _ = allClearVoteBallotAccepted(feedFetchResult{
		ok: true, alert: "AllClear", metricsOK: true, lhl: 1, di: 0, ad: 2,
	})
	if !ok {
		t.Fatal("AllClear with AD activity must count")
	}

	ok, detail := allClearVoteBallotAccepted(feedFetchResult{
		ok: false, failureClass: "telemetry_collapse", errMsg: "hold", alert: "Unknown",
	})
	if ok || !strings.Contains(detail, "telemetry_collapse") {
		t.Fatalf("sticky/collapse must not count, got ok=%v detail=%s", ok, detail)
	}

	ok, _ = allClearVoteBallotAccepted(feedFetchResult{
		ok: true, alert: "Warning", metricsOK: true, di: 3, ad: 2,
	})
	if ok {
		t.Fatal("non-AllClear must not count")
	}
}

func TestApplyTelemetryStickyHoldAndRestore(t *testing.T) {
	tr := &LightningTrigger{
		feedHealth: map[string]*FeedHealth{},
		failover:   defaultLightningFailoverPolicy(),
	}
	elevatedXML := `<?xml version="1.0"?><thordata><lhl>6</lhl><di>3.2</di><ad>5</ad><lightningalert>RedAlert</lightningalert></thordata>`
	floorXML := `<?xml version="1.0"?><thordata><lhl>0</lhl><di>0.0</di><ad>0</ad><lightningalert>AllClear</lightningalert></thordata>`
	diRestoreXML := `<?xml version="1.0"?><thordata><lhl>0</lhl><di>0.5</di><ad>0</ad><lightningalert>AllClear</lightningalert></thordata>`
	adRestoreXML := `<?xml version="1.0"?><thordata><lhl>1</lhl><di>0.0</di><ad>2</ad><lightningalert>Caution</lightningalert></thordata>`

	// Seed elevated history
	r1 := tr.applyTelemetryToResult("primary", feedFetchResult{ok: true, alert: "RedAlert", xmlString: elevatedXML})
	if !r1.ok {
		t.Fatal("elevated sample should be ok")
	}

	// Cliff
	r2 := tr.applyTelemetryToResult("primary", feedFetchResult{ok: true, alert: "AllClear", xmlString: floorXML})
	if r2.ok || r2.failureClass != "telemetry_collapse" {
		t.Fatalf("expected cliff failure, got ok=%v class=%s", r2.ok, r2.failureClass)
	}
	if !tr.feedHasTelemetryCollapse("primary") {
		t.Fatal("expected sticky collapse after cliff")
	}

	// Next floor poll must stay held (the poll-N+1 gap)
	r3 := tr.applyTelemetryToResult("primary", feedFetchResult{ok: true, alert: "AllClear", xmlString: floorXML})
	if r3.ok || r3.failureClass != "telemetry_collapse" {
		t.Fatalf("expected sticky hold, got ok=%v class=%s err=%s", r3.ok, r3.failureClass, r3.errMsg)
	}
	if !tr.feedHasTelemetryCollapse("primary") {
		t.Fatal("sticky must remain on floor AllClear")
	}

	// DI decimal restore
	r4 := tr.applyTelemetryToResult("primary", feedFetchResult{ok: true, alert: "AllClear", xmlString: diRestoreXML})
	if !r4.ok {
		t.Fatalf("DI restore should clear sticky, got err=%s", r4.errMsg)
	}
	if tr.feedHasTelemetryCollapse("primary") {
		t.Fatal("sticky should clear on DI > 0")
	}

	// Re-collapse path: elevate then floor again, then AD restore
	_ = tr.applyTelemetryToResult("primary", feedFetchResult{ok: true, alert: "RedAlert", xmlString: elevatedXML})
	_ = tr.applyTelemetryToResult("primary", feedFetchResult{ok: true, alert: "AllClear", xmlString: floorXML})
	if !tr.feedHasTelemetryCollapse("primary") {
		t.Fatal("expected collapse again")
	}
	r5 := tr.applyTelemetryToResult("primary", feedFetchResult{ok: true, alert: "Caution", xmlString: adRestoreXML})
	if !r5.ok || tr.feedHasTelemetryCollapse("primary") {
		t.Fatalf("AD restore should clear sticky, ok=%v sticky=%v err=%s", r5.ok, tr.feedHasTelemetryCollapse("primary"), r5.errMsg)
	}
}
