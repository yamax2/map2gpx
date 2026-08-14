package main

import (
	"os"
	"testing"
	"time"
)

// buildRecord assembles one record from its fields as the camera writes them,
// in the form parseRecord takes: the file's ASCII text XOR 0x30, which is what
// XOR 0xAA (the obfuscation) and XOR 0x9A (this tool's key) leave behind.
//
// Fields are written at their fixed offsets, so a test case names only what it
// is about and the rest stays a valid record around it.
func buildRecord(utc, latHemi, lat, lonHemi, lon, altSign, alt, speed string) []byte {
	rec := make([]byte, recordRead)
	for i := range rec {
		rec[i] = '0' ^ 0x30
	}

	put := func(offset int, value string) {
		for i := 0; i < len(value); i++ {
			rec[offset+i] = value[i] ^ 0x30
		}
	}

	put(offLocalTime, utc) // the local clock is not read; a copy keeps the record realistic
	put(offUTCTime, utc)
	put(offLatHemi, latHemi)
	put(offLat, lat)
	put(offLonHemi, lonHemi)
	put(offLon, lon)
	put(offAltSign, altSign)
	put(offAlt, alt)
	put(offSpeed, speed)

	return rec
}

// TestParseRecord covers the decoding of one record, field by field. Every
// "wrong width" case here is a bug this tool shipped before v0.2.1: reading the
// longitude as 8 of its 9 digits, the altitude as 3 of its 4, and the hemisphere
// markers not at all.
func TestParseRecord(t *testing.T) {
	const (
		utc  = "20151219120305"
		lat  = "58165611"  // 58°16.5611'
		lon  = "054562581" // 054°56.2581'
		alt  = "0142"
		spd  = "064"
		zero = "00000000"
	)

	tests := []struct {
		name      string
		rec       []byte
		wantOK    bool
		wantLat   float64
		wantLon   float64
		wantAlt   int
		wantSpeed float64
		wantWhen  string
	}{
		{
			name: "a northern, eastern record", rec: buildRecord(utc, "N", lat, "E", lon, "+", alt, spd),
			wantOK: true, wantLat: 58.2760183, wantLon: 54.9376350, wantAlt: 142, wantSpeed: 64,
			wantWhen: "2015-12-19T12:03:05Z",
		},
		{
			// The whole field is nine digits wide; reading eight of them decoded
			// this as 10.927.
			name: "a longitude past 100 degrees", rec: buildRecord(utc, "N", lat, "E", "105562581", "+", alt, spd),
			wantOK: true, wantLat: 58.2760183, wantLon: 105.9376350, wantAlt: 142, wantSpeed: 64,
			wantWhen: "2015-12-19T12:03:05Z",
		},
		{
			name: "a southern latitude", rec: buildRecord(utc, "S", lat, "E", lon, "+", alt, spd),
			wantOK: true, wantLat: -58.2760183, wantLon: 54.9376350, wantAlt: 142, wantSpeed: 64,
			wantWhen: "2015-12-19T12:03:05Z",
		},
		{
			name: "a western longitude", rec: buildRecord(utc, "N", lat, "W", lon, "+", alt, spd),
			wantOK: true, wantLat: 58.2760183, wantLon: -54.9376350, wantAlt: 142, wantSpeed: 64,
			wantWhen: "2015-12-19T12:03:05Z",
		},
		{
			// Four digits wide; reading three of them decoded this as 234.
			name: "an altitude past 1000 metres", rec: buildRecord(utc, "N", lat, "E", lon, "+", "1234", spd),
			wantOK: true, wantLat: 58.2760183, wantLon: 54.9376350, wantAlt: 1234, wantSpeed: 64,
			wantWhen: "2015-12-19T12:03:05Z",
		},
		{
			name: "an altitude below sea level", rec: buildRecord(utc, "N", lat, "E", lon, "-", "0042", spd),
			wantOK: true, wantLat: 58.2760183, wantLon: 54.9376350, wantAlt: -42, wantSpeed: 64,
			wantWhen: "2015-12-19T12:03:05Z",
		},
		{
			// Three digits wide, in km/h. Read as four it would only work while
			// the byte after it stays '0'.
			name: "a three-digit speed", rec: buildRecord(utc, "N", lat, "E", lon, "+", alt, "144"),
			wantOK: true, wantLat: 58.2760183, wantLon: 54.9376350, wantAlt: 142, wantSpeed: 144,
			wantWhen: "2015-12-19T12:03:05Z",
		},
		{
			name: "a record with no fix yet", rec: buildRecord(utc, "N", zero, "E", "000000000", "+", "0000", "000"),
		},
		{
			name: "minutes at 60, which cannot occur", rec: buildRecord(utc, "N", "58600000", "E", lon, "+", alt, spd),
		},
		{
			name: "an unknown hemisphere marker", rec: buildRecord(utc, "X", lat, "E", lon, "+", alt, spd),
		},
		{
			name: "a non-digit in a digit field", rec: buildRecord(utc, "N", "581656X1", "E", lon, "+", alt, spd),
		},
		{
			name: "a month of zero", rec: buildRecord("20150019120305", "N", lat, "E", lon, "+", alt, spd),
		},
		{
			name: "an hour past the day", rec: buildRecord("20151219250305", "N", lat, "E", lon, "+", alt, spd),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := parseRecord(test.rec)
			if ok != test.wantOK {
				t.Fatalf("ok = %v, want %v", ok, test.wantOK)
			}
			if !test.wantOK {
				return
			}

			if !closeEnough(got.lat, test.wantLat) {
				t.Errorf("lat = %.7f, want %.7f", got.lat, test.wantLat)
			}
			if !closeEnough(got.lon, test.wantLon) {
				t.Errorf("lon = %.7f, want %.7f", got.lon, test.wantLon)
			}
			if got.alt != test.wantAlt {
				t.Errorf("alt = %d, want %d", got.alt, test.wantAlt)
			}
			if got.speed != test.wantSpeed {
				t.Errorf("speed = %.1f, want %.1f", got.speed, test.wantSpeed)
			}
			if when := got.when.Format(time.RFC3339); when != test.wantWhen {
				t.Errorf("when = %s, want %s", when, test.wantWhen)
			}
		})
	}
}

// TestParseRecordReadsTheGPSClock pins which of the record's two clocks is used.
// The camera writes its own RTC first and the GPS fix's UTC second, and the RTC
// is the one that drifts.
func TestParseRecordReadsTheGPSClock(t *testing.T) {
	rec := buildRecord("20151219120305", "N", "58165611", "E", "054562581", "+", "0142", "064")
	for i, b := range []byte("20151219170311") {
		rec[offLocalTime+i] = b ^ 0x30
	}

	got, ok := parseRecord(rec)
	if !ok {
		t.Fatal("record rejected")
	}

	if when := got.when.Format(time.RFC3339); when != "2015-12-19T12:03:05Z" {
		t.Errorf("when = %s, want the GPS clock 2015-12-19T12:03:05Z, not the RTC", when)
	}
}

// TestScanRecordsFixture decodes the committed AMBA0004.MAP, which is a real
// log: one record per second, so its point count is also the length in seconds
// of the clip it was recorded beside.
func TestScanRecordsFixture(t *testing.T) {
	data, err := os.ReadFile("../AMBA0004.MAP")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	track := scanRecords(data)

	if len(track) != 3027 {
		t.Fatalf("decoded %d points, want 3027", len(track))
	}

	first, last := track[0], track[len(track)-1]

	if when := first.when.Format(time.RFC3339); when != "2016-01-01T07:34:35Z" {
		t.Errorf("first point at %s, want 2016-01-01T07:34:35Z", when)
	}
	if !closeEnough(first.lat, 56.8102033) || !closeEnough(first.lon, 59.5341700) {
		t.Errorf("first point at %.7f,%.7f, want 56.8102033,59.5341700", first.lat, first.lon)
	}
	if first.alt != 336 || first.speed != 61 {
		t.Errorf("first point %dm at %.1f km/h, want 336m at 61.0 km/h", first.alt, first.speed)
	}

	// One record per second, so the track spans one second less than its length.
	if span := last.when.Sub(first.when); span != time.Duration(len(track)-1)*time.Second {
		t.Errorf("track spans %s over %d points, want one point per second", span, len(track))
	}
}

// TestScanRecordsRejectsGarbage checks that a "TT" outside a record does not
// become a trackpoint, which is what the per-field digit checks are for.
func TestScanRecordsRejectsGarbage(t *testing.T) {
	data := []byte("TT" + string(make([]byte, recordRead)))

	if track := scanRecords(data); len(track) != 0 {
		t.Errorf("decoded %d points from garbage, want 0", len(track))
	}
}

// closeEnough compares coordinates at the precision the GPX output carries.
func closeEnough(got, want float64) bool {
	const epsilon = 5e-8

	return got-want < epsilon && want-got < epsilon
}
