package generate

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reCamerasHeader = regexp.MustCompile(`^cameras:`)
	reTopLevel      = regexp.MustCompile(`^[a-z]`)
	reCameraName    = regexp.MustCompile(`^\s{2}(\w[\w-]*):`)
	reStreamsHeader  = regexp.MustCompile(`^\s{2}streams:`)
	reStreamName    = regexp.MustCompile(`^\s{4}'?(\w[\w-]*)'?:`)
	reStreamContent = regexp.MustCompile(`^\s{4,}`)
	reNextSection   = regexp.MustCompile(`^[a-z#]`)
	reCameraBody    = regexp.MustCompile(`^\s{2,}\S`)
	reVersion       = regexp.MustCompile(`^version:`)
)

// addToConfig inserts a new camera into existing frigate YAML
func addToConfig(existing string, info *cameraInfo, req *Request) (*Response, error) {
	lines := strings.Split(existing, "\n")

	// Step 1. Collect existing names for deduplication
	existingCams := findNames(lines, reCamerasHeader, reCameraName)
	existingStreams := findNames(lines, reStreamsHeader, reStreamName)

	// Step 2. Deduplicate names
	info = dedup(info, existingCams, existingStreams)

	// Step 3. Find insertion points
	streamIdx := findStreamInsertPoint(lines)
	cameraIdx := findCameraInsertPoint(lines)

	if streamIdx == -1 || cameraIdx == -1 {
		return nil, fmt.Errorf("generate: can't find go2rtc streams or cameras section")
	}

	// Step 4. Build lines to insert
	var sb strings.Builder
	writeStreamLines(&sb, info)
	streamLines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")

	sb.Reset()
	writeCameraBlock(&sb, info, req)
	cameraLines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")

	// Step 5. Splice
	added := make(map[int]bool)
	result := make([]string, 0, len(lines)+len(streamLines)+len(cameraLines))

	result = append(result, lines[:streamIdx]...)
	mark := len(result)
	result = append(result, streamLines...)
	for i := range streamLines {
		added[mark+i] = true
	}

	// adjust camera index after stream insertion
	shift := len(streamLines)
	adjCameraIdx := cameraIdx + shift
	rest := lines[streamIdx:]
	split := adjCameraIdx - len(result)

	result = append(result, rest[:split]...)
	mark = len(result)
	result = append(result, cameraLines...)
	for i := range cameraLines {
		added[mark+i] = true
	}
	result = append(result, rest[split:]...)

	config := strings.Join(result, "\n")
	diff := diffWithContext(result, added, 3)
	return &Response{Config: config, Diff: diff}, nil
}

func dedup(info *cameraInfo, cams, streams map[string]bool) *cameraInfo {
	// copy to avoid mutating original
	out := *info

	suffix := 0
	base := out.CameraName
	for cams[out.CameraName] {
		suffix++
		out.CameraName = fmt.Sprintf("%s_%d", base, suffix)
	}

	base = out.MainStreamName
	for streams[out.MainStreamName] {
		suffix++
		out.MainStreamName = fmt.Sprintf("%s_%d", base, suffix)
	}

	if out.SubStreamName != "" {
		base = out.SubStreamName
		for streams[out.SubStreamName] {
			suffix++
			out.SubStreamName = fmt.Sprintf("%s_%d", base, suffix)
		}
	}

	return &out
}

// findNames extracts names from a YAML section
func findNames(lines []string, header, nameRe *regexp.Regexp) map[string]bool {
	names := make(map[string]bool)
	in := false
	for _, line := range lines {
		if header.MatchString(line) {
			in = true
			continue
		}
		if in && reTopLevel.MatchString(line) {
			break
		}
		if in {
			if m := nameRe.FindStringSubmatch(line); m != nil {
				names[m[1]] = true
			}
		}
	}
	return names
}

func findStreamInsertPoint(lines []string) int {
	in := false
	last := -1
	headerIdx := -1
	for i, line := range lines {
		if reStreamsHeader.MatchString(line) {
			in = true
			headerIdx = i
			continue
		}
		if in {
			if reStreamContent.MatchString(line) {
				last = i
			} else if reNextSection.MatchString(line) {
				if last >= 0 && last+1 < len(lines) && strings.TrimSpace(lines[last+1]) == "" {
					return last + 2
				}
				if last >= 0 {
					return last + 1
				}
				// empty streams section -- insert right after header
				return headerIdx + 1
			}
		}
	}
	if last >= 0 {
		return last + 1
	}
	// empty streams section at end of file
	if headerIdx >= 0 {
		return headerIdx + 1
	}
	return -1
}

func findCameraInsertPoint(lines []string) int {
	in := false
	last := -1
	headerIdx := -1
	for i, line := range lines {
		if reCamerasHeader.MatchString(line) {
			in = true
			headerIdx = i
			continue
		}
		if in {
			if reCameraBody.MatchString(line) {
				last = i
			} else if reTopLevel.MatchString(line) && !reCamerasHeader.MatchString(line) {
				if last < 0 {
					// empty cameras section -- insert right after header
					return headerIdx + 1
				}
				idx := last + 1
				for idx < len(lines) && strings.TrimSpace(lines[idx]) == "" {
					idx++
				}
				return idx
			} else if reVersion.MatchString(line) {
				if last < 0 {
					return headerIdx + 1
				}
				idx := i
				for idx > 0 && strings.TrimSpace(lines[idx-1]) == "" {
					idx--
				}
				return idx
			}
		}
	}
	if headerIdx >= 0 {
		return headerIdx + 1
	}
	return len(lines)
}
