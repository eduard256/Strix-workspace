package generate

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// supported schemes that go2rtc handles natively
var supported = map[string]bool{
	"rtsp": true, "rtsps": true, "rtspx": true,
	"rtmp": true, "rtmps": true, "rtmpx": true,
	"http": true, "https": true, "httpx": true,
	"onvif": true, "bubble": true, "webrtc": true,
	"ffmpeg": true, "exec": true, "hass": true,
	"homekit": true, "dvrip": true, "tapo": true,
	"kasa": true, "gopro": true, "nest": true,
	"ring": true, "wyze": true, "isapi": true,
	"ivideon": true, "roborock": true, "tuya": true,
}

// schemes where frigate needs ?mp4 suffix on restream path
var needMP4 = map[string]bool{"bubble": true}

var reIPv4 = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)

// Generate builds a complete frigate YAML config with one camera added.
// If existingConfig is empty, creates config from scratch.
func Generate(req *Request) (*Response, error) {
	// Step 1. Validate
	if req.MainStream == "" {
		return nil, fmt.Errorf("generate: mainStream required")
	}

	scheme := urlScheme(req.MainStream)
	if !supported[scheme] {
		return nil, fmt.Errorf("generate: unsupported scheme: %s", scheme)
	}

	if req.SubStream != "" {
		if s := urlScheme(req.SubStream); !supported[s] {
			return nil, fmt.Errorf("generate: unsupported sub scheme: %s", s)
		}
	}

	// Step 2. Build camera info
	info := buildInfo(req)

	// Step 3. Ensure detect is enabled when objects are set
	if len(req.Objects) > 0 && (req.Detect == nil || !req.Detect.Enabled) {
		if req.Detect == nil {
			req.Detect = &DetectConfig{Enabled: true}
		} else {
			req.Detect.Enabled = true
		}
	}

	// Step 4. Generate config
	if strings.TrimSpace(req.ExistingConfig) == "" {
		config := newConfig(info, req)
		return &Response{Config: config, Diff: fullDiff(config)}, nil
	}

	return addToConfig(req.ExistingConfig, info, req)
}

// buildInfo resolves all names and paths, applying overrides from request
func buildInfo(req *Request) *cameraInfo {
	mainScheme := urlScheme(req.MainStream)
	ip := extractIP(req.MainStream)
	sanitized := strings.NewReplacer(".", "_", ":", "_").Replace(ip)

	base := "camera"
	streamBase := "stream"
	if ip != "" {
		base = "camera_" + sanitized
		streamBase = sanitized
	}

	info := &cameraInfo{
		CameraName:     base,
		MainStreamName: streamBase + "_main",
		MainSource:     req.MainStream,
	}

	// apply name override
	if req.Name != "" {
		info.CameraName = req.Name
		info.MainStreamName = req.Name + "_main"
	}

	// apply go2rtc overrides
	if req.Go2RTC != nil {
		if req.Go2RTC.MainStreamName != "" {
			info.MainStreamName = req.Go2RTC.MainStreamName
		}
		if req.Go2RTC.MainStreamSource != "" {
			info.MainSource = req.Go2RTC.MainStreamSource
		}
	}

	// main stream frigate path
	info.MainPath = "rtsp://127.0.0.1:8554/" + info.MainStreamName
	if needMP4[mainScheme] {
		info.MainPath += "?mp4"
	}
	info.MainInputArgs = "preset-rtsp-restream"

	// apply frigate overrides
	if req.Frigate != nil {
		if req.Frigate.MainStreamPath != "" {
			info.MainPath = req.Frigate.MainStreamPath
		}
		if req.Frigate.MainStreamInputArgs != "" {
			info.MainInputArgs = req.Frigate.MainStreamInputArgs
		}
	}

	// sub stream
	if req.SubStream != "" {
		subScheme := urlScheme(req.SubStream)
		subName := streamBase + "_sub"
		if req.Name != "" {
			subName = req.Name + "_sub"
		}
		subSource := req.SubStream
		subPath := "rtsp://127.0.0.1:8554/" + subName
		if needMP4[subScheme] {
			subPath += "?mp4"
		}
		subInputArgs := "preset-rtsp-restream"

		if req.Go2RTC != nil {
			if req.Go2RTC.SubStreamName != "" {
				subName = req.Go2RTC.SubStreamName
			}
			if req.Go2RTC.SubStreamSource != "" {
				subSource = req.Go2RTC.SubStreamSource
			}
		}
		if req.Frigate != nil {
			if req.Frigate.SubStreamPath != "" {
				subPath = req.Frigate.SubStreamPath
			}
			if req.Frigate.SubStreamInputArgs != "" {
				subInputArgs = req.Frigate.SubStreamInputArgs
			}
		}

		info.SubStreamName = subName
		info.SubSource = subSource
		info.SubPath = subPath
		info.SubInputArgs = subInputArgs
	}

	return info
}

// newConfig builds a complete frigate YAML from scratch
func newConfig(info *cameraInfo, req *Request) string {
	var b strings.Builder

	b.WriteString("mqtt:\n  enabled: false\n\n")
	b.WriteString("record:\n  enabled: true\n  retain:\n    days: 7\n    mode: motion\n\n")

	// go2rtc
	b.WriteString("go2rtc:\n  streams:\n")
	writeStreamLines(&b, info)

	// cameras
	b.WriteString("cameras:\n")
	writeCameraBlock(&b, info, req)

	b.WriteString("version: 0.18-0\n")
	return b.String()
}

// internals

type cameraInfo struct {
	CameraName     string
	MainStreamName string
	MainSource     string
	MainPath       string
	MainInputArgs  string
	SubStreamName  string
	SubSource      string
	SubPath        string
	SubInputArgs   string
}

func urlScheme(rawURL string) string {
	if i := strings.IndexByte(rawURL, ':'); i > 0 {
		return rawURL[:i]
	}
	return ""
}

func extractIP(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	if m := reIPv4.FindString(rawURL); m != "" {
		return m
	}
	return ""
}
