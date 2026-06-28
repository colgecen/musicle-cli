package music

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ytInitialPlayerResponse JSON structures

type playerResponse struct {
	VideoDetails   *videoDetails   `json:"videoDetails"`
	StreamingData  *streamingData  `json:"streamingData"`
	PlayabilityStatus *playabilityStatus `json:"playabilityStatus"`
}

type videoDetails struct {
	Title          string     `json:"title"`
	LengthSeconds  string     `json:"lengthSeconds"`
	Author         string     `json:"author"`
	ChannelID      string     `json:"channelId"`
	ShortDescription string   `json:"shortDescription"`
	Thumbnail      *thumbnails `json:"thumbnail"`
	IsPrivate      bool       `json:"isPrivate"`
	IsLiveContent  bool       `json:"isLiveContent"`
}

type thumbnails struct {
	Thumbnails []thumbnail `json:"thumbnails"`
}

type thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type streamingData struct {
	ExpiresInSeconds string          `json:"expiresInSeconds"`
	Formats          []streamFormat  `json:"formats"`
	AdaptiveFormats  []streamFormat  `json:"adaptiveFormats"`
}

type streamFormat struct {
	ITag             int    `json:"itag"`
	MimeType         string `json:"mimeType"`
	Bitrate          int    `json:"bitrate"`
	ContentLength    string `json:"contentLength"`
	URL              string `json:"url"`
	SignatureCipher  string `json:"signatureCipher"`
	Cipher           string `json:"cipher"`
	AudioQuality     string `json:"audioQuality"`
	AudioSampleRate  string `json:"audioSampleRate"`
	AudioChannels    int    `json:"audioChannels"`
	FPS              int    `json:"fps"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	QualityLabel     string `json:"qualityLabel"`
}

type playabilityStatus struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// extractInitialPlayerResponse extracts ytInitialPlayerResponse JSON from HTML.
// Returns the raw JSON string and any error.
func extractInitialPlayerResponse(html string) (string, error) {
	// Match: var ytInitialPlayerResponse = {...};
	re := regexp.MustCompile(`ytInitialPlayerResponse\s*=\s*({.*?});`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		// Sometimes it's in a script tag with JSON inside
		re2 := regexp.MustCompile(`"playerResponse":"({.*?})"`)
		matches2 := re2.FindStringSubmatch(html)
		if len(matches2) < 2 {
			return "", fmt.Errorf("ytInitialPlayerResponse not found in HTML")
		}
		return matches2[1], nil
	}
	return matches[1], nil
}

// ParsePlayerResponse parses the ytInitialPlayerResponse JSON into a structured result.
type ParseResult struct {
	Title        string
	Author       string
	DurationSec  float64
	ThumbnailURL string
	Streams      []StreamInfo
}

// StreamInfo represents a single audio stream option.
type StreamInfo struct {
	ITag         int
	MimeType     string
	Bitrate      int
	ContentLen   int64
	URL          string
	AudioQuality string
	SampleRate   int
	Channels     int
	Format       string // "webm" or "m4a"
}

// signatureCipherFields parses a signatureCipher query string into components.
type cipherFields struct {
	URL string
	S   string
	SP  string
}

// resolveStreamURL returns the actual stream URL, handling signatureCipher if needed.
func resolveStreamURL(f streamFormat) (string, error) {
	if f.URL != "" {
		return f.URL, nil
	}

	cipher := f.SignatureCipher
	if cipher == "" {
		cipher = f.Cipher
	}
	if cipher == "" {
		return "", fmt.Errorf("no URL and no signatureCipher")
	}

	// Parse signatureCipher: s=...&sp=sig&url=https://...
	parsed, err := url.ParseQuery(cipher)
	if err != nil {
		return "", fmt.Errorf("parse signatureCipher: %w", err)
	}

	streamURL := parsed.Get("url")
	s := parsed.Get("s")
	sp := parsed.Get("sp")
	if sp == "" {
		sp = "signature"
	}

	if streamURL == "" {
		return "", fmt.Errorf("signatureCipher missing url parameter")
	}

	if s != "" {
		// Append signature to URL — this is a simplified approach.
		// Full deciphering requires running JS from base.js player.
		if strings.Contains(streamURL, "?") {
			streamURL += "&" + sp + "=" + url.QueryEscape(s)
		} else {
			streamURL += "?" + sp + "=" + url.QueryEscape(s)
		}
	}

	return streamURL, nil
}

// ParsePlayerResponse extracts video details and audio streams from the HTML.
func ParsePlayerResponse(html string) (*ParseResult, error) {
	rawJSON, err := extractInitialPlayerResponse(html)
	if err != nil {
		return nil, err
	}

	// Unescape JSON if it was embedded in a string
	if strings.HasPrefix(rawJSON, "\\\"") {
		var unescaped string
		if err := json.Unmarshal([]byte(`"`+rawJSON+`"`), &unescaped); err == nil {
			rawJSON = unescaped
		}
	}

	var pr playerResponse
	if err := json.Unmarshal([]byte(rawJSON), &pr); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w\nraw: %s", err, rawJSON[:min(len(rawJSON), 200)])
	}

	if pr.PlayabilityStatus != nil && pr.PlayabilityStatus.Status != "OK" {
		reason := pr.PlayabilityStatus.Reason
		if reason == "" {
			reason = pr.PlayabilityStatus.Status
		}
		return nil, fmt.Errorf("video not playable: %s", reason)
	}

	if pr.VideoDetails == nil {
		return nil, fmt.Errorf("missing videoDetails in player response")
	}

	if pr.StreamingData == nil || len(pr.StreamingData.AdaptiveFormats) == 0 {
		return nil, fmt.Errorf("no adaptive formats available")
	}

	// Duration
	durSec, _ := strconv.ParseFloat(pr.VideoDetails.LengthSeconds, 64)

	// Thumbnail
	thumbURL := ""
	if pr.VideoDetails.Thumbnail != nil && len(pr.VideoDetails.Thumbnail.Thumbnails) > 0 {
		thumbURL = pr.VideoDetails.Thumbnail.Thumbnails[len(pr.VideoDetails.Thumbnail.Thumbnails)-1].URL
	}

	// Collect audio-only adaptive formats
	var streams []StreamInfo
	for _, f := range pr.StreamingData.AdaptiveFormats {
		mime := f.MimeType
		// Only audio-only (no video)
		if strings.Contains(mime, "video/") {
			continue
		}

		streamURL, err := resolveStreamURL(f)
		if err != nil {
			continue // skip streams we can't resolve
		}

		var fmtName string
		if strings.Contains(mime, "webm") || strings.Contains(mime, "opus") {
			fmtName = "webm"
		} else if strings.Contains(mime, "mp4") || strings.Contains(mime, "m4a") || strings.Contains(mime, "aac") {
			fmtName = "m4a"
		} else {
			continue
		}

		contentLen, _ := strconv.ParseInt(f.ContentLength, 10, 64)
		sampleRate, _ := strconv.Atoi(f.AudioSampleRate)

		streams = append(streams, StreamInfo{
			ITag:         f.ITag,
			MimeType:     mime,
			Bitrate:      f.Bitrate,
			ContentLen:   contentLen,
			URL:          streamURL,
			AudioQuality: f.AudioQuality,
			SampleRate:   sampleRate,
			Channels:     f.AudioChannels,
			Format:       fmtName,
		})
	}

	if len(streams) == 0 {
		return nil, fmt.Errorf("no audio-only streams found")
	}

	return &ParseResult{
		Title:        pr.VideoDetails.Title,
		Author:       pr.VideoDetails.Author,
		DurationSec:  durSec,
		ThumbnailURL: thumbURL,
		Streams:      streams,
	}, nil
}

// BestAudioStream picks the best audio stream (highest bitrate, prefer opus/webm).
func BestAudioStream(streams []StreamInfo) *StreamInfo {
	if len(streams) == 0 {
		return nil
	}

	best := &streams[0]
	for i := 1; i < len(streams); i++ {
		s := &streams[i]
		// Prefer opus/webm over m4a
		if s.Format == "webm" && best.Format == "m4a" {
			best = s
			continue
		}
		// Higher bitrate is better
		if s.Bitrate > best.Bitrate && s.Format == best.Format {
			best = s
		}
	}
	return best
}
