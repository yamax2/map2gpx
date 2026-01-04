package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

const (
	RecordSize = 16
	Offset     = 1
	Scale      = 1e7
)

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

	for i := 0; i+Offset+12 <= len(data); i += RecordSize {
		rec := data[i+Offset:]

		latRaw := int32(binary.LittleEndian.Uint32(rec[0:4]))
		lonRaw := int32(binary.LittleEndian.Uint32(rec[4:8]))
		ts := binary.LittleEndian.Uint32(rec[8:12])

		lat := float64(latRaw) / Scale
		lon := float64(lonRaw) / Scale

		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			continue
		}

		t := time.Unix(int64(ts), 0).UTC().Format(time.RFC3339)

		fmt.Fprintf(out,
			`<trkpt lat="%.7f" lon="%.7f"><time>%s</time></trkpt>`+"\n",
			lat, lon, t,
		)
		points++
	}

	fmt.Fprintln(out, `</trkseg></trk></gpx>`)

	fmt.Printf("Done. %d track points written\n", points)
}
