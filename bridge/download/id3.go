package download

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

// ID3v2.3 tag writer — pure Go, no dependencies.

// id3PaddingSize is the number of zero bytes appended after the tag frames.
const id3PaddingSize = 2048

func syncsafeEncode(n uint32) []byte {
	return []byte{
		byte((n >> 21) & 0x7f),
		byte((n >> 14) & 0x7f),
		byte((n >> 7) & 0x7f),
		byte(n & 0x7f),
	}
}

// id3Unsynchronise replaces bytes that could form false MP3 sync (FFh) with
// FFh 00h sequences. Returns (unsynchronised data, wasModified, error).
func id3Unsynchronise(data []byte) ([]byte, bool) {
	var out bytes.Buffer
	modified := false
	for i := 0; i < len(data); i++ {
		out.WriteByte(data[i])
		if data[i] == 0xFF && i+1 < len(data) && (data[i+1]&0xE0) != 0 {
			// 0xFF followed by a byte with high 3 bits set → insert 0x00
			// Actually, ID3 unsynchronisation: 0xFF 0x00 → 0xFF 0x00 0x00
			if data[i+1] == 0x00 {
				out.WriteByte(0x00)
				modified = true
			}
		}
	}
	return out.Bytes(), modified
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
// Adds padding (2048 bytes) and unsynchronisation to prevent false MP3 sync words.
func WriteID3Tag(mp3Data []byte, info *TrackInfo) ([]byte, error) {
	tagLen := uint32(0)
	tag := &bytes.Buffer{}

	frames := [][]byte{
		writeTextFrame("TIT2", info.Title),
		writeTextFrame("TPE1", info.Artist),
		writeTextFrame("TALB", info.Album),
		writeNumFrame("TRCK", info.TrackNum),
	}

	if info.StreamURL != "" {
		comm := writeCommentFrame("eng", "Source", info.StreamURL)
		if comm != nil {
			frames = append(frames, comm)
		}
		frames = append(frames, writeTextFrame("WOAS", info.StreamURL))
	}

	var txxxFrames [][]byte
	if info.Playlist != "" {
		txxxFrames = append(txxxFrames, writeTXXXFrame("Playlist", info.Playlist))
	}
	if info.Thumbnail != "" {
		txxxFrames = append(txxxFrames, writeTXXXFrame("Thumbnail", info.Thumbnail))
	}
	txxxFrames = append(txxxFrames, writeTXXXFrame("Encoding", "MusicLeCLI pure Go encoder"))
	frames = append(frames, txxxFrames...)

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

	// Unsynchronise frame data
	rawTag, _ := id3Unsynchronise(tag.Bytes())
	// Add padding
	pad := id3PaddingSize
	totalSize := len(rawTag) + pad

	header := make([]byte, 10)
	copy(header[0:3], "ID3")
	header[3] = 3
	header[4] = 0
	// Set unsynchronisation flag (bit 7)
	header[5] = 0x80
	copy(header[6:10], syncsafeEncode(uint32(totalSize)))

	var out bytes.Buffer
	out.Write(header)
	out.Write(rawTag)
	// Write padding zeros
	zeros := make([]byte, pad)
	out.Write(zeros)
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
