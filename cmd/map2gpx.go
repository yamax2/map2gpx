package main

import (
	"fmt"
	"os"
	"time"
)

const XORKey = 0x9A

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: map2gpx input.MAP output.gpx")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	out, err := os.Create(os.Args[2])
	if err != nil {
		panic(err)
	}
	defer out.Close()

	// GPX header
	fmt.Fprintln(out, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(out, `<gpx version="1.1" creator="showme-map-converter" xmlns="http://www.topografix.com/GPX/1/1">`)
	fmt.Fprintln(out, `<trk><trkseg>`)

	points := 0

	// Find "TT" markers which separate records
	for i := 0; i < len(data)-60; i++ {
		if data[i] != 'T' || data[i+1] != 'T' {
			continue
		}

		// Decode record (XOR with 0x9A)
		rec := make([]byte, 60)
		for j := 0; j < 60; j++ {
			rec[j] = data[i+2+j] ^ XORKey
		}

		// Parse timestamp from BCD digits at positions 0-13
		// Format: YYYY MM DD HH MM SS (each digit is one byte)
		year := bcdToInt(rec[0:4])
		month := bcdToInt(rec[4:6])
		day := bcdToInt(rec[6:8])
		hour := bcdToInt(rec[8:10])
		min := bcdToInt(rec[10:12])
		sec := bcdToInt(rec[12:14])

		// Parse latitude from BCD at positions 31-38
		// Format: ddmm.mmmm (NMEA style)
		latRaw := bcdToInt(rec[31:39])
		latDeg := latRaw / 1000000
		latMin := float64(latRaw%1000000) / 10000.0
		lat := float64(latDeg) + latMin/60.0

		// Parse longitude from BCD at positions 40-48
		// Format: dddmm.mmmm or ddmm.mmmm (NMEA style)
		lonRaw := bcdToInt(rec[40:48])
		var lon float64
		if lonRaw >= 10000000 {
			// dddmm.mmmm format
			lonDeg := lonRaw / 1000000
			lonMin := float64(lonRaw%1000000) / 10000.0
			lon = float64(lonDeg) + lonMin/60.0
		} else {
			// ddmm.mmmm format
			lonDeg := lonRaw / 100000
			lonMin := float64(lonRaw%100000) / 1000.0
			lon = float64(lonDeg) + lonMin/60.0
		}

		// Validate coordinates
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			continue
		}
		if lat == 0 && lon == 0 {
			continue
		}

		t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)

		fmt.Fprintf(out,
			`<trkpt lat="%.7f" lon="%.7f"><time>%s</time></trkpt>`+"\n",
			lat, lon, t.Format(time.RFC3339),
		)
		points++
	}

	fmt.Fprintln(out, `</trkseg></trk></gpx>`)

	fmt.Printf("Done. %d track points written\n", points)
}

// bcdToInt converts a slice of BCD digits to an integer
func bcdToInt(digits []byte) int {
	result := 0
	for _, d := range digits {
		if d <= 9 {
			result = result*10 + int(d)
		}
	}
	return result
}
