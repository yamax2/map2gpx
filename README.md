# map2gpx

A command-line tool to convert `.MAP` GPS log files from SHOW-ME dashcams (AMBA format) to standard GPX format.

## About

SHOW-ME dashcams store GPS tracks in a proprietary `.MAP` binary format alongside video files. This tool decodes and converts them to GPX, which can be used with mapping software, Google Earth, or video geotagging tools.

### MAP File Format

The AMBA MAP format uses:
- `TT` markers as record separators (251 bytes apart)
- XOR encoding (key: `0x9A`)
- BCD-encoded timestamps and coordinates
- NMEA-style coordinate format (`ddmm.mmmm`)

## Installation

```bash
go install github.com/user/map2gpx/cmd/map2gpx@latest
```

Or build from source:

```bash
go build -o map2gpx ./cmd/map2gpx.go
```

## Usage

```bash
map2gpx input.MAP output.gpx
```

## Example

```bash
$ map2gpx AMBA0004.MAP track.gpx
Done. 3027 track points written
```

Output:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="showme-map-converter" xmlns="http://www.topografix.com/GPX/1/1">
<trk><trkseg>
<trkpt lat="56.8102033" lon="59.5341667"><time>2016-01-01T12:34:56Z</time></trkpt>
<trkpt lat="56.8102350" lon="59.5338833"><time>2016-01-01T12:34:57Z</time></trkpt>
...
</trkseg></trk></gpx>
```
