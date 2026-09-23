package main

import (
	"testing"
	"time"
)

func testHoursCFG() OperatingHoursConfig {
	cfg := defaultOperatingHours()
	cfg.Timezone = "America/New_York"
	return cfg
}

func nyTime(y int, m time.Month, d, hh, mm int) time.Time {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return time.Date(y, m, d, hh, mm, 0, 0, loc)
}

func TestNthWeekdayOccurrenceJulDec2026(t *testing.T) {
	// 3rd Sat/Sun 2026 from TARR public schedule
	cases := []struct {
		day  time.Time
		want int
	}{
		{nyTime(2026, time.July, 18, 12, 0), 3},    // 3rd Sat Jul
		{nyTime(2026, time.July, 19, 12, 0), 3},    // 3rd Sun Jul
		{nyTime(2026, time.August, 15, 12, 0), 3},  // 3rd Sat Aug
		{nyTime(2026, time.August, 16, 12, 0), 3},  // 3rd Sun Aug
		{nyTime(2026, time.September, 19, 12, 0), 3},
		{nyTime(2026, time.September, 20, 12, 0), 3},
		{nyTime(2026, time.October, 17, 12, 0), 3},
		{nyTime(2026, time.October, 18, 12, 0), 3},
		{nyTime(2026, time.November, 21, 12, 0), 3}, // 3rd Sat Nov
		{nyTime(2026, time.November, 15, 12, 0), 3}, // 3rd Sun Nov
		{nyTime(2026, time.December, 19, 12, 0), 3},
		{nyTime(2026, time.December, 20, 12, 0), 3},
		{nyTime(2026, time.July, 11, 12, 0), 2}, // 2nd Sat
		{nyTime(2026, time.July, 25, 12, 0), 4}, // 4th Sat
	}
	for _, tc := range cases {
		got := nthWeekdayOccurrence(tc.day)
		if got != tc.want {
			t.Errorf("%s: nth=%d want %d", tc.day.Format("2006-01-02"), got, tc.want)
		}
	}
}

func TestCalendarOpenThirdWeekend(t *testing.T) {
	cfg := testHoursCFG()
	if !isCalendarOpenDay(cfg, nyTime(2026, time.July, 18, 12, 0)) {
		t.Fatal("3rd Sat Jul should be open")
	}
	if !isCalendarOpenDay(cfg, nyTime(2026, time.July, 19, 12, 0)) {
		t.Fatal("3rd Sun Jul should be open")
	}
	if isCalendarOpenDay(cfg, nyTime(2026, time.July, 11, 12, 0)) {
		t.Fatal("2nd Sat Jul should not be open")
	}
	if isCalendarOpenDay(cfg, nyTime(2026, time.July, 20, 12, 0)) {
		t.Fatal("Mon after 3rd weekend should not be open")
	}
}

func TestCalendarLegacyNoRules(t *testing.T) {
	cfg := testHoursCFG()
	cfg.CalendarRules = nil
	if !isCalendarOpenDay(cfg, nyTime(2026, time.July, 11, 12, 0)) {
		t.Fatal("no rules → all days allowed")
	}
}

func TestQuietHoursOvernight(t *testing.T) {
	cfg := testHoursCFG()
	cfg.QuietHours = QuietHoursConfig{Enabled: true, Start: "22:00", End: "06:00"}

	if !isQuietHours(cfg, nyTime(2026, time.July, 18, 23, 0)) {
		t.Fatal("23:00 should be quiet")
	}
	if !isQuietHours(cfg, nyTime(2026, time.July, 19, 3, 0)) {
		t.Fatal("03:00 should be quiet")
	}
	if isQuietHours(cfg, nyTime(2026, time.July, 18, 21, 59)) {
		t.Fatal("21:59 should not be quiet")
	}
	if isQuietHours(cfg, nyTime(2026, time.July, 19, 6, 0)) {
		t.Fatal("06:00 should not be quiet (end exclusive)")
	}
}

func TestLightningAudioWindow(t *testing.T) {
	cfg := testHoursCFG()

	// Public Sat noon → operating
	if got := lightningAudioWindow(cfg, nyTime(2026, time.July, 18, 12, 0)); got != AudioWindowOperating {
		t.Fatalf("Sat 12:00 want operating got %s", got)
	}
	// Public Sat after close → after_hours
	if got := lightningAudioWindow(cfg, nyTime(2026, time.July, 18, 17, 0)); got != AudioWindowAfterHours {
		t.Fatalf("Sat 17:00 want after_hours got %s", got)
	}
	// Any day 03:00 → quiet
	if got := lightningAudioWindow(cfg, nyTime(2026, time.July, 14, 3, 0)); got != AudioWindowQuiet {
		t.Fatalf("Tue 03:00 want quiet got %s", got)
	}
	// Non-public weekday afternoon → after_hours
	if got := lightningAudioWindow(cfg, nyTime(2026, time.July, 14, 14, 0)); got != AudioWindowAfterHours {
		t.Fatalf("Tue 14:00 want after_hours got %s", got)
	}
}

func TestWithinOperatingRequiresCalendar(t *testing.T) {
	cfg := testHoursCFG()
	// Saturday enabled 10-16 but not 3rd weekend
	if isWithinOperatingHours(cfg, nyTime(2026, time.July, 11, 12, 0)) {
		t.Fatal("2nd Sat should not be within operating hours")
	}
	if !isWithinOperatingHours(cfg, nyTime(2026, time.July, 18, 12, 0)) {
		t.Fatal("3rd Sat noon should be within operating hours")
	}
	if isWithinOperatingHours(cfg, nyTime(2026, time.July, 18, 17, 0)) {
		t.Fatal("3rd Sat after close should not be within operating hours")
	}
}
