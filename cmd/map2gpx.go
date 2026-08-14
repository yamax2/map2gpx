package main

import (
	"fmt"
	"os"
	"time"
)

// XORKey deobfuscates a record. The file is really ASCII text stored under XOR
// 0xAA; XORing with 0x9A instead turns each digit character into its own value
// ('0'^0xAA^0x9A == 0), which is what lets the fields below be read as BCD. A
// non-digit marker therefore arrives as its character XOR 0x30, hence the marker
// constants.
const XORKey = 0x9A

const (
	markerNorth = 'N' ^ 0x30
	markerSouth = 'S' ^ 0x30
	markerEast  = 'E' ^ 0x30
	markerWest  = 'W' ^ 0x30
	markerMinus = '-' ^ 0x30
)

// Record layout, in bytes from the end of the "TT" marker that separates
// records. Every field is fixed width, and reading one by its width is the whole
// trick: a 9-digit longitude read as 8 digits turns 105°56.2581' into 10.927°.
const (
	recordLen  = 251 // distance between two "TT" markers
	recordRead = 57  // bytes of it this tool reads

	offLocalTime = 0  // 14 digits — the camera's RTC clock, ignored in favour of the GPS one
	offUTCTime   = 15 // 14 digits, YYYYMMDDhhmmss
	offLatHemi   = 30 // 'N' or 'S'
	offLat       = 31 // 8 digits, ddmm.mmmm
	offLonHemi   = 39 // 'E' or 'W'
	offLon       = 40 // 9 digits, dddmm.mmmm
	offAltSign   = 49 // '+' or '-'
	offAlt       = 50 // 4 digits, metres
	offSpeed     = 54 // 3 digits, km/h
)

// record is one decoded GPS record.
type record struct {
	when  time.Time
	lat   float64
	lon   float64
	alt   int
	speed float64
}

// version is stamped at build time with -ldflags "-X main.version=<tag>"; an
// unstamped build says so rather than claiming a release it is not.
//
// It is worth having because two builds of this tool decode the same card
// differently: everything up to v0.2.0 read three fixed-width fields at the
// wrong width, so a track from one and a track from the other disagree — mildly
// in the north-east quadrant of the world, wildly outside it.
var version = "dev"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Println("map2gpx", version)
		return
	}

	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: map2gpx input.MAP > output.gpx")
		fmt.Fprintln(os.Stderr, "       map2gpx -version")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "map2gpx: %v\n", err)
		os.Exit(1)
	}

	out := os.Stdout

	// GPX header
	fmt.Fprintln(out, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(out, `<gpx version="1.1" creator="showme-map-converter" xmlns="http://www.topografix.com/GPX/1/1">`)
	fmt.Fprintln(out, `<trk><trkseg>`)

	track := scanRecords(data)

	for _, point := range track {
		fmt.Fprintf(out,
			"<trkpt lat=\"%.7f\" lon=\"%.7f\">"+
				"<ele>%d</ele>"+
				"<time>%s</time>"+
				"<speed>%.1f</speed>"+
				"</trkpt>\n",
			point.lat, point.lon, point.alt, point.when.Format(time.RFC3339), point.speed,
		)
	}

	fmt.Fprintln(out, `</trkseg></trk></gpx>`)

	fmt.Fprintf(os.Stderr, "Done. %d track points written\n", len(track))
}

// scanRecords decodes every record in a log, skipping the ones parseRecord
// rejects.
//
// It finds them by their "TT" marker, scanning byte by byte rather than striding
// recordLen: no field of a record can hold a 'T', so a marker is the only thing
// that matches, and a truncated or partly overwritten log stays readable from
// wherever its records resume.
func scanRecords(data []byte) []record {
	var track []record

	for i := 0; i+2+recordRead <= len(data); i++ {
		if data[i] != 'T' || data[i+1] != 'T' {
			continue
		}

		rec := make([]byte, recordRead)
		for j := range rec {
			rec[j] = data[i+2+j] ^ XORKey
		}

		point, ok := parseRecord(rec)
		if !ok {
			continue
		}

		track = append(track, point)
	}

	return track
}

// parseRecord decodes one record. ok is false when a fixed-width digit field
// does not hold digits (a truncated log, or a "TT" that is not a marker after
// all), when the timestamp fields are out of range, or when the coordinates are
// unusable — including the (0, 0) a record written before the first fix carries.
func parseRecord(rec []byte) (record, bool) {
	when, ok := parseTime(rec[offUTCTime : offUTCTime+14])
	if !ok {
		return record{}, false
	}

	// ddmm.mmmm and dddmm.mmmm: the leading two or three digits are degrees and
	// the remaining six are minutes to four decimals.
	lat, ok := parseCoord(rec[offLat:offLat+8], 2)
	if !ok {
		return record{}, false
	}
	lon, ok := parseCoord(rec[offLon:offLon+9], 3)
	if !ok {
		return record{}, false
	}

	// The hemisphere markers are the only thing that makes the southern and
	// western halves of the world representable; the digits are unsigned.
	switch rec[offLatHemi] {
	case markerNorth:
	case markerSouth:
		lat = -lat
	default:
		return record{}, false
	}

	switch rec[offLonHemi] {
	case markerEast:
	case markerWest:
		lon = -lon
	default:
		return record{}, false
	}

	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return record{}, false
	}
	if lat == 0 && lon == 0 {
		return record{}, false
	}

	alt, ok := bcdToInt(rec[offAlt : offAlt+4])
	if !ok {
		return record{}, false
	}
	// Only '-' is looked for, where the hemisphere markers above reject anything
	// they do not recognise: getting a hemisphere wrong puts the point on the
	// other side of the equator, while an unreadable altitude sign costs a metre
	// reading nothing else depends on — not the whole record's position.
	if rec[offAltSign] == markerMinus {
		alt = -alt
	}

	speed, ok := bcdToInt(rec[offSpeed : offSpeed+3])
	if !ok {
		return record{}, false
	}

	return record{when: when, lat: lat, lon: lon, alt: alt, speed: float64(speed)}, true
}

// parseTime decodes the 14-digit YYYYMMDDhhmmss field as UTC. Out-of-range parts
// are rejected rather than normalised, since time.Date would silently turn a
// half-written record's month 0 into December of the year before.
func parseTime(field []byte) (time.Time, bool) {
	parts := make([]int, 0, 6)
	for _, width := range []int{4, 2, 2, 2, 2, 2} {
		value, ok := bcdToInt(field[:width])
		if !ok {
			return time.Time{}, false
		}
		parts = append(parts, value)
		field = field[width:]
	}

	year, month, day, hour, min, sec := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]
	if month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || min > 59 || sec > 59 {
		return time.Time{}, false
	}

	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC), true
}

// parseCoord decodes one fixed-width NMEA coordinate: degDigits digits of
// degrees followed by six digits of minutes (mm.mmmm), unsigned.
//
// Minutes at or past 60 are rejected. They cannot occur in the format, and the
// degree range check the caller applies would not catch them: 99.9999 minutes
// would quietly add 1.67° to the degrees and still land somewhere on Earth.
func parseCoord(field []byte, degDigits int) (float64, bool) {
	degrees, ok := bcdToInt(field[:degDigits])
	if !ok {
		return 0, false
	}

	minutes, ok := bcdToInt(field[degDigits:])
	if !ok || minutes >= 60*10000 {
		return 0, false
	}

	return float64(degrees) + float64(minutes)/10000.0/60.0, true
}

// bcdToInt converts a slice of BCD digits to an integer. ok is false if any byte
// is not a digit: the fields are fixed width, so a non-digit means the record is
// not one, and skipping the byte the way a lenient reader would would shift
// every digit after it into the wrong place.
func bcdToInt(digits []byte) (int, bool) {
	result := 0
	for _, d := range digits {
		if d > 9 {
			return 0, false
		}
		result = result*10 + int(d)
	}

	return result, true
}
