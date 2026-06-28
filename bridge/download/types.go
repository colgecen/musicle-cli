package download

// TrackInfo holds metadata for a single downloadable track.
type TrackInfo struct {
	Title       string
	Artist      string
	Album       string
	DurationSec float64
	StreamURL   string
	Format      string // "webm" (opus) or "m4a" (aac)
	ContentLen  int64
	Thumbnail   string
	Playlist    string // playlist name if part of one
	TrackNum    int    // position in playlist
}

// PlaylistInfo holds metadata for a playlist download.
type PlaylistInfo struct {
	Name   string
	Tracks []TrackInfo
}

// ProgressCallback is called during download/decode/encode with a percent and message.
type ProgressCallback func(pct int, msg string)
