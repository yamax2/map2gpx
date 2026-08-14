# map2gpx

A command-line tool to convert `.MAP` GPS log files from SHOW-ME dashcams (AMBA format) to standard GPX format.

## About

SHOW-ME dashcams store GPS tracks in a proprietary `.MAP` binary format alongside video files. This tool decodes and converts them to GPX, which can be used with mapping software, Google Earth, or video geotagging tools.

### MAP File Format

A `.MAP` file is ASCII text obfuscated with **XOR `0xAA`** — the header decodes as `MEDIA 1.` and the trailer carries the clip's own file name and byte size. Records are 251 bytes apart, each introduced by the byte pair `0x54 0x54` (`TT` as stored, which is not text under the obfuscation: it would decode to `0xFE 0xFE`).

This tool XORs with **`0x9A`** instead, which is `0xAA ^ 0x30`: that turns every digit *character* into its own value, so the numeric fields can be read as BCD. A non-digit marker byte therefore arrives as its character XOR `0x30`.

One record, in bytes from the end of its `TT` marker — **every field is fixed width, and must be read by its width**:

| Offset | Width | Contents |
|--------|-------|----------|
| 0 | 14 | Local time `YYYYMMDDhhmmss` — the camera's RTC clock |
| 14 | 1 | separator |
| 15 | 14 | **GPS UTC** time `YYYYMMDDhhmmss` — what this tool emits |
| 29 | 1 | separator |
| 30 | 1 | `N` or `S` |
| 31 | 8 | Latitude `ddmm.mmmm` (NMEA style, unsigned) |
| 39 | 1 | `E` or `W` |
| 40 | 9 | Longitude `dddmm.mmmm` (NMEA style, unsigned) |
| 49 | 1 | `+` or `-` |
| 50 | 4 | Altitude in metres |
| 54 | 3 | Speed in km/h |
| 57 | … | zero padding to 251 bytes |

Both clocks are present in every record; the GPS one is used, being the accurate half (on the reference unit the RTC ran 2–6 s ahead of it).

The record layout above was verified against 29,550 records from a two-day trip. Three earlier bugs all came from not reading a field at its stated width, or at all: a longitude read as 8 digits instead of 9 turned `E105°56.2581'` into `10.927°` (and truncated the last digit of every smaller longitude), an altitude read as 3 digits instead of 4 turned 1234 m into 234 m, and the `N`/`S` and `E`/`W` markers were never read at all, which put the southern and western hemispheres on the wrong side of the equator and the prime meridian.

Hence the strictness now: a field whose bytes are not all digits rejects the whole record rather than skipping the offending byte and shifting every digit after it, an unrecognised hemisphere marker rejects it too, and minutes at or past 60 are rejected — they cannot occur, and a degree range check does not catch them.

## Installation

```bash
go install github.com/yamax2/map2gpx/cmd/map2gpx@latest
```

Or build from source:

```bash
go build -o map2gpx ./cmd/
go test ./...            # decodes the committed AMBA0004.MAP, plus a record per field
```

## Usage

```bash
map2gpx input.MAP > output.gpx
map2gpx -version
```

The GPX is written to stdout; the status message is written to stderr.

**Check the version before trusting a track.** Builds up to `v0.2.0` decode coordinates wrongly (see [MAP File Format](#map-file-format)), and the two agree closely enough in the north-east quadrant of the world that the difference is easy to miss: there, only the last digit of each longitude changes. A build from source with no `-ldflags` stamp reports `dev`.

## Example

```bash
$ map2gpx AMBA0004.MAP > track.gpx
Done. 3027 track points written
```

Output — one `<trkpt>` per second, in one `<trk>` of one `<trkseg>`, with `<ele>` in metres and `<speed>` in **km/h** (not the m/s a GPX 1.1 extension would use). There is no `<course>`: the record carries no heading.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="showme-map-converter" xmlns="http://www.topografix.com/GPX/1/1">
<trk><trkseg>
<trkpt lat="56.8102033" lon="59.5341700"><ele>336</ele><time>2016-01-01T07:34:35Z</time><speed>61.0</speed></trkpt>
<trkpt lat="56.8102350" lon="59.5338983"><ele>336</ele><time>2016-01-01T07:34:36Z</time><speed>60.0</speed></trkpt>
...
</trkseg></trk></gpx>
```

A record is skipped when a fixed-width digit field does not hold digits, when the timestamp parts are out of range, or when the coordinates are unusable — including the `(0, 0)` written before the receiver's first fix.
