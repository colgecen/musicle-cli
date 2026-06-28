package music

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// extractVideoID parses a YouTube URL and returns the video ID.
// Supports:
//
//	https://www.youtube.com/watch?v=VIDEO_ID
//	https://youtu.be/VIDEO_ID
//	https://m.youtube.com/watch?v=VIDEO_ID
//	https://youtube.com/watch?v=VIDEO_ID
func extractVideoID(url string) (string, error) {
	// youtu.be short links
	if strings.Contains(url, "youtu.be") {
		parts := strings.Split(url, "/")
		for i, p := range parts {
			if strings.Contains(p, "youtu.be") && i+1 < len(parts) {
				id := parts[i+1]
				// strip query params
				if idx := strings.Index(id, "?"); idx >= 0 {
					id = id[:idx]
				}
				if idx := strings.Index(id, "&"); idx >= 0 {
					id = id[:idx]
				}
				if len(id) == 11 {
					return id, nil
				}
			}
		}
		return "", fmt.Errorf("cannot extract video ID from youtu.be URL: %s", url)
	}

	// /watch?v=VIDEO_ID
	if strings.Contains(url, "watch?v=") {
		idx := strings.Index(url, "watch?v=")
		id := url[idx+8:]
		if amp := strings.Index(id, "&"); amp >= 0 {
			id = id[:amp]
		}
		if len(id) == 11 {
			return id, nil
		}
		return "", fmt.Errorf("invalid video ID length in /watch URL: %s", url)
	}

	return "", fmt.Errorf("unsupported YouTube URL format: %s", url)
}

// FetchYouTubePage fetches the HTML page for a YouTube video.
// Accepts a full URL or video ID.
func FetchYouTubePage(urlOrID string) (string, error) {
	videoID := urlOrID
	if strings.HasPrefix(urlOrID, "http") {
		var err error
		videoID, err = extractVideoID(urlOrID)
		if err != nil {
			return "", err
		}
	} else if len(videoID) != 11 {
		return "", fmt.Errorf("invalid video ID: %q", videoID)
	}

	watchURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	req, err := http.NewRequest("GET", watchURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return string(body), nil
}
