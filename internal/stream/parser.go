package stream

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func LoadPool(path string) (*Pool, error) {
	file, fileError := os.Open(path)
	if fileError != nil {
		return nil, fmt.Errorf("opening the playlist: %w (SEGMENTS_DIR has to name the directory holding segment.m3u8 and its .ts files)", fileError)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	if !scanner.Scan() {
		return nil, fmt.Errorf("segment.m3u8 is empty")
	}
	// First read
	if scanner.Text() != "#EXTM3U" {
		return nil, fmt.Errorf("segment.m3u8 is invalid: missing #EXTM3U header")
	}

	/* Parsing header to check metadata*/
	firstSegmentLine, err := parseHeader(scanner)
	if err != nil {
		return nil, err
	}

	segments, maxDuration, err := parseSegments(scanner, firstSegmentLine)
	if err != nil {
		return nil, err
	}

	/*The playlist lives next to the media it names*/
	if err := verifySegments(filepath.Dir(path), segments); err != nil {
		return nil, err
	}

	pool := &Pool{
		Segments: segments,
		/*HLS RFC 8216 must be an integer*/
		TargetDuration: int(math.Ceil(maxDuration)),
	}

	return pool, nil
}

/*
A playlist reachable at SEGMENTS_DIR says nothing about the media next to it:
a partial unzip, or a directory holding the .m3u8 alone, parses 64 perfectly
valid segments with nothing behind them. Without this the server boots, answers
the playlist with a 200 and 404s every segment, and the player spins forever
with nothing to report. Better to refuse to start and say which file is missing
*/
func verifySegments(dir string, segments []Segment) error {
	for _, segment := range segments {
		/*
			os.Stat only asks the filesystem for the entry's metadata, it never opens
			the file, so checking all 64 at boot costs nothing
		*/
		if _, err := os.Stat(filepath.Join(dir, segment.Name)); err != nil {
			return fmt.Errorf("%s is listed in the playlist but missing from %q: all %d .ts files have to sit next to segment.m3u8", segment.Name, dir, len(segments))
		}
	}

	return nil
}

func parseHeader(scanner *bufio.Scanner) (firstSegmentLine string, err error) {
	sawVersion := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#EXTINF:") {
			if !sawVersion {
				return "", fmt.Errorf("segment.m3u8 is invalid: missing #EXT-X-VERSION")
			}
			return line, nil
		}

		if matched, versionErr := checkVersionLine(line); matched {
			if versionErr != nil {
				return "", versionErr
			}
			sawVersion = true
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return "", scanErr
	}

	return "", fmt.Errorf("segment.m3u8 is invalid: no segments found")
}

func parseSegments(scanner *bufio.Scanner, firstSegmentLine string) (segments []Segment, maxDuration float64, err error) {
	line := firstSegmentLine

	for {
		/*End of m3u8 file*/
		if line == "#EXT-X-ENDLIST" {
			break
		}

		/*Are we in segments sections? */
		durationText, ok := strings.CutPrefix(line, "#EXTINF:")
		if ok {
			/*Get rid of trailing comma*/
			durationText = strings.TrimSuffix(durationText, ",")
			/*Transform to float given that format commes with .000000*/
			duration, parseErr := strconv.ParseFloat(durationText, 64)
			if parseErr != nil {
				return nil, 0, parseErr
			}

			/*If no line after #EXTINF there is a format error */
			if !scanner.Scan() {
				return nil, 0, fmt.Errorf("segment.m3u8 is truncated: missing filename after %q", line)
			}

			segments = append(segments, Segment{Name: scanner.Text(), Duration: duration})
			if duration > maxDuration {
				maxDuration = duration
			}
		}

		if !scanner.Scan() {
			break
		}
		line = scanner.Text()
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return nil, 0, scanErr
	}

	return segments, maxDuration, nil
}

func checkVersionLine(line string) (matched bool, err error) {
	versionText, ok := strings.CutPrefix(line, "#EXT-X-VERSION:")
	if !ok {
		return false, nil
	}

	version, convErr := strconv.Atoi(versionText)
	if convErr != nil {
		return true, convErr
	}
	if version != SupportedVersion {
		return true, fmt.Errorf("unsupported HLS version: %d (only %d is supported)", version, SupportedVersion)
	}

	return true, nil
}
