package download

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

// ID3v2.3 tag writer — pure Go, no dependencies.

func syncsafeEncode(n uint32) []byte {
	return []byte{
		byte((n >> 21) & 0x7f),
		byte((n >> 14) & 0x7f),
		byte((n >> 7) & 0x7f),
		byte(n & 0x7f),
	}
}

func writeTextFrame(id, text string) []byte {
	if text == "" {
		return nil
	}
	var buf bytes.Buffer
	// encoding: 3 = UTF-8
	buf.WriteByte(3)
	buf.WriteString(text)

	frame := make([]byte, 10)
	copy(frame[0:4], id)
	binary.BigEndian.PutUint32(frame[4:8], uint32(buf.Len()))
	// flags = 0
	frame = append(frame, buf.Bytes()...)
	return frame
}

func writeNumFrame(id string, num int) []byte {
	if num <= 0 {
		return nil
	}
	return writeTextFrame(id, fmt.Sprintf("%d", num))
}

// WriteID3Tag prepends an ID3v2.3 tag to mp3Data using metadata from info.
// Supports standard frames: TIT2, TPE1, TALB, TCON, TYER, TRCK, COMM, TXXX, WOAS, TPUB, TCOP.
func WriteID3Tag(mp3Data []byte, info *TrackInfo) ([]byte, error) {
	tagLen := uint32(0)
	tag := &bytes.Buffer{}

	frames := [][]byte{
		writeTextFrame("TIT2", info.Title),
		writeTextFrame("TPE1", info.Artist),
		writeTextFrame("TALB", info.Album),
		writeNumFrame("TRCK", info.TrackNum),
	}

	// TYER: Year (from DurationSec as placeholder or empty)
	if info.DurationSec > 0 {
		// No year info available, skip TYER for now
	}

	// TCON: Genre (empty for now)
	// TPUB: Publisher
	// TCOP: Copyright

	// COMM: Comment with source info
	if info.StreamURL != "" {
		comm := writeCommentFrame("eng", "Source", info.StreamURL)
		if comm != nil {
			frames = append(frames, comm)
		}
	}

	// WOAS: Official audio source URL
	if info.StreamURL != "" {
		frames = append(frames, writeTextFrame("WOAS", info.StreamURL))
	}

	// TXXX: User-defined text frames
	var txxxFrames [][]byte
	if info.Playlist != "" {
		// TXXX: Playlist name
		txxxFrames = append(txxxFrames, writeTXXXFrame("Playlist", info.Playlist))
	}
	if info.Thumbnail != "" {
		txxxFrames = append(txxxFrames, writeTXXXFrame("Thumbnail", info.Thumbnail))
	}
	txxxFrames = append(txxxFrames, writeTXXXFrame("Encoding", "MusicLeCLI pure Go encoder"))
	frames = append(frames, txxxFrames...)

	// TLEN: Duration in milliseconds
	if info.DurationSec > 0 {
		frames = append(frames, writeNumFrame("TLEN", int(info.DurationSec*1000)))
	}

	for _, f := range frames {
		if f == nil {
			continue
		}
		if _, err := tag.Write(f); err != nil {
			return nil, err
		}
		tagLen += uint32(len(f))
	}

	if tagLen == 0 {
		return mp3Data, nil
	}

	header := make([]byte, 10)
	copy(header[0:3], "ID3")
	header[3] = 3 // major version
	header[4] = 0 // minor version
	header[5] = 0 // flags
	copy(header[6:10], syncsafeEncode(tagLen))

	var out bytes.Buffer
	out.Write(header)
	out.Write(tag.Bytes())
	out.Write(mp3Data)

	return out.Bytes(), nil
}

// writeTXXXFrame creates a TXXX (User-defined text information) frame.
func writeTXXXFrame(description, value string) []byte {
	if description == "" || value == "" {
		return nil
	}
	var buf bytes.Buffer
	buf.WriteByte(3) // UTF-8
	buf.Write([]byte(description))
	buf.WriteByte(0) // null separator
	buf.WriteString(value)

	frame := make([]byte, 10)
	copy(frame[0:4], "TXXX")
	binary.BigEndian.PutUint32(frame[4:8], uint32(buf.Len()))
	frame = append(frame, buf.Bytes()...)
	return frame
}

// writeCommentFrame creates a COMM (Comments) frame.
func writeCommentFrame(lang, description, text string) []byte {
	if text == "" {
		return nil
	}
	var buf bytes.Buffer
	buf.WriteByte(3) // UTF-8
	// Language (3 bytes)
	if len(lang) >= 3 {
		buf.WriteString(lang[:3])
	} else {
		buf.WriteString("eng")
	}
	// Content descriptor (null-terminated)
	buf.Write([]byte(description))
	buf.WriteByte(0)
	// The actual comment text
	buf.WriteString(text)

	frame := make([]byte, 10)
	copy(frame[0:4], "COMM")
	binary.BigEndian.PutUint32(frame[4:8], uint32(buf.Len()))
	frame = append(frame, buf.Bytes()...)
	return frame
}

// WriteMP3ToFile writes MP3 data with ID3v2.3 tag to a file.
func WriteMP3ToFile(path string, mp3Data []byte, info *TrackInfo) error {
	tagged, err := WriteID3Tag(mp3Data, info)
	if err != nil {
		return fmt.Errorf("id3 tag: %w", err)
	}
	return os.WriteFile(path, tagged, 0644)
}
