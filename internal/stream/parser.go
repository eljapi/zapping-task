package stream

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

func LoadPool(path string) (*Pool, error) {
	file, fileError := os.Open(path)
	if fileError != nil {
		return nil, fileError
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

	pool := &Pool{
		Segments: segments,
		/*HLS RFC 8216 must be an integer*/
		TargetDuration: int(math.Ceil(maxDuration)),
	}

	return pool, nil
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
