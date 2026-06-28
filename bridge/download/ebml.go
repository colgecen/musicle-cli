package download

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// EBML element IDs (32-bit with VINT encoding).
const (
	EBMLID          uint32 = 0x1A45DFA3
	SegmentID       uint32 = 0x18538067
	InfoID          uint32 = 0x1549A966
	TracksID        uint32 = 0x1654AE6B
	ClusterID       uint32 = 0x1F43B675
	SeekHeadID      uint32 = 0x114D9B74
	CuesID          uint32 = 0x1C53BB6B
	TagsID          uint32 = 0x1254C367
)

// EBML info children.
const (
	TimecodeScaleID uint32 = 0x2AD7B1
	DurationID      uint32 = 0x4489
)

// Track entry children.
const (
	TrackEntryID    uint32 = 0xAE
	TrackNumberID   uint32 = 0xD7
	TrackTypeID     uint32 = 0x83
	CodecIDID       uint32 = 0x86
	AudioID         uint32 = 0xE1
	SamplingFreqID  uint32 = 0xB5
	ChannelsID      uint32 = 0x9F
)

// Cluster children.
const (
	ClusterTimecodeID uint32 = 0xE7
	SimpleBlockID     uint32 = 0xA3
	BlockGroupID      uint32 = 0xA0
	BlockID           uint32 = 0xA1
	BlockDurationID   uint32 = 0x9B
	ReferenceBlockID  uint32 = 0xFB
)

// Track types.
const (
	TrackTypeVideo    = 1
	TrackTypeAudio    = 2
	TrackTypeComplex  = 3
	TrackTypeSubtitle = 16
)

// EBMLAudioTrack holds parsed audio track info.
type EBMLAudioTrack struct {
	TrackNumber    int
	CodecID        string // "A_OPUS", "A_VORBIS", etc.
	SampleRate     float64
	Channels       int
}

// EBMLInfo holds parsed segment info.
type EBMLInfo struct {
	TimecodeScale int64 // nanoseconds per tick
	Duration      float64
}

// EBMLCluster holds timecode and block data.
type EBMLCluster struct {
	Timecode      int64
	SimpleBlocks  []EBMLSimpleBlock
}

// EBMLSimpleBlock holds a single audio/video frame.
type EBMLSimpleBlock struct {
	TrackNumber int
	Timecode    int16 // relative to cluster timecode
	Keyframe    bool
	Data        []byte
}

// vintSize returns the encoded size of a VINT given its first byte.
func vintSize(first byte) int {
	for i := 0; i < 8; i++ {
		if first&(0x80>>i) != 0 {
			return i + 1
		}
	}
	return 8
}

// readVINT reads a variable-length integer from the reader.
// Returns the decoded value and the number of bytes consumed.
func readVINT(r io.Reader) (uint64, int, error) {
	var first [1]byte
	if _, err := io.ReadFull(r, first[:]); err != nil {
		return 0, 0, err
	}
	size := vintSize(first[0])
	if size == 1 {
		return uint64(first[0] & 0x7F), 1, nil
	}
	raw := make([]byte, size)
	raw[0] = first[0]
	if _, err := io.ReadFull(r, raw[1:]); err != nil {
		return 0, 0, err
	}
	var val uint64
	for i := 0; i < size; i++ {
		val = (val << 8) | uint64(raw[i])
	}
	// Clear the VINT marker bit (most significant bit of first byte)
	marker := byte(0x80 >> (size - 1))
	val &^= uint64(marker) << (8 * (size - 1))
	return val, size, nil
}

// readElementID reads a variable-length element ID from the reader.
func readElementID(r io.Reader) (uint32, int, error) {
	val, n, err := readVINT(r)
	if err != nil {
		return 0, 0, err
	}
	return uint32(val), n, nil
}

// readElementSize reads a variable-length data size from the reader.
func readElementSize(r io.Reader) (uint64, int, error) {
	return readVINT(r)
}

// EBMLReader reads and navigates EBML/WebM structures.
type EBMLReader struct {
	data    []byte
	offset  int
}

// NewEBMLReader creates a new EBML reader from raw WebM bytes.
func NewEBMLReader(data []byte) *EBMLReader {
	return &EBMLReader{data: data}
}

// readUint reads an unsigned integer of n bytes from the data.
func (r *EBMLReader) readUint(n int) (uint64, error) {
	if r.offset+n > len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}
	var val uint64
	for i := 0; i < n; i++ {
		val = (val << 8) | uint64(r.data[r.offset+i])
	}
	r.offset += n
	return val, nil
}

// readFloat reads a float of size bytes (4 or 8).
func (r *EBMLReader) readFloat(size int) (float64, error) {
	if r.offset+size > len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}
	var val float64
	switch size {
	case 4:
		bits := binary.BigEndian.Uint32(r.data[r.offset:])
		val = float64(math.Float32frombits(bits))
		r.offset += 4
	case 8:
		bits := binary.BigEndian.Uint64(r.data[r.offset:])
		val = math.Float64frombits(bits)
		r.offset += 8
	default:
		return 0, fmt.Errorf("unsupported float size: %d", size)
	}
	return val, nil
}

// readString reads a string of the given length.
func (r *EBMLReader) readString(size int) (string, error) {
	if r.offset+size > len(r.data) {
		return "", io.ErrUnexpectedEOF
	}
	s := string(r.data[r.offset : r.offset+size])
	r.offset += size
	return s, nil
}

// skip advances the offset by n bytes.
func (r *EBMLReader) skip(n int) {
	r.offset += n
}

// ParseAudioTracks parses the Tracks element and returns audio track info.
func (r *EBMLReader) ParseAudioTracks() ([]EBMLAudioTrack, error) {
	if r.offset >= len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}

	br := bytes.NewReader(r.data[r.offset:])
	id, idLen, err := readElementID(br)
	if err != nil {
		return nil, fmt.Errorf("read tracks id: %w", err)
	}
	if id != TracksID {
		return nil, fmt.Errorf("expected Tracks element (0x%X), got 0x%X", TracksID, id)
	}
	size, sizeLen, err := readElementSize(br)
	if err != nil {
		return nil, fmt.Errorf("read tracks size: %w", err)
	}

	headerLen := idLen + sizeLen
	r.offset += headerLen

	return r.parseTracksBody(int(size))
}

func (r *EBMLReader) parseTracksBody(size int) ([]EBMLAudioTrack, error) {
	end := r.offset + size
	var tracks []EBMLAudioTrack

	for r.offset < end {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br)
		if err != nil {
			return tracks, err
		}
		elemSize, sizeLen, err := readElementSize(br)
		if err != nil {
			return tracks, err
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		if id == TrackEntryID {
			track, err := r.parseTrackEntry(dataSize)
			if err != nil {
				return tracks, err
			}
			if track != nil {
				tracks = append(tracks, *track)
			}
		} else {
			r.offset += totalHeader + dataSize
		}
	}
	return tracks, nil
}

func (r *EBMLReader) parseTrackEntry(size int) (*EBMLAudioTrack, error) {
	end := r.offset + size
	var track EBMLAudioTrack
	var isAudio bool

	for r.offset < end {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br)
		if err != nil {
			return nil, err
		}
		elemSize, sizeLen, err := readElementSize(br)
		if err != nil {
			return nil, err
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		switch id {
		case TrackNumberID:
			val, _ := r.readUint(dataSize)
			track.TrackNumber = int(val)
		case TrackTypeID:
			val, _ := r.readUint(dataSize)
			isAudio = (int(val) == TrackTypeAudio)
		case CodecIDID:
			codec, _ := r.readString(dataSize)
			track.CodecID = codec
		case AudioID:
			r.parseAudioSettings(&track, dataSize)
		default:
			r.offset += totalHeader + dataSize
			continue
		}
	}

	if !isAudio {
		return nil, nil
	}
	return &track, nil
}

func (r *EBMLReader) parseAudioSettings(track *EBMLAudioTrack, size int) {
	end := r.offset + size
	for r.offset < end {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, _ := readElementID(br)
		elemSize, sizeLen, _ := readElementSize(br)
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		switch id {
		case SamplingFreqID:
			if dataSize == 4 {
				bits := binary.BigEndian.Uint32(r.data[r.offset:])
				track.SampleRate = float64(math.Float32frombits(bits))
				r.offset += dataSize
			} else if dataSize == 8 {
				bits := binary.BigEndian.Uint64(r.data[r.offset:])
				track.SampleRate = math.Float64frombits(bits)
				r.offset += dataSize
			} else {
				r.offset += dataSize
			}
		case ChannelsID:
			val, _ := r.readUint(dataSize)
			track.Channels = int(val)
		default:
			r.offset += totalHeader + dataSize
		}
	}
}

// ParseSegmentInfo parses the Info element for timecode scale and duration.
func (r *EBMLReader) ParseSegmentInfo(size int) (*EBMLInfo, error) {
	end := r.offset + size
	info := &EBMLInfo{TimecodeScale: 1000000} // default 1ms

	for r.offset < end {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br)
		if err != nil {
			return info, err
		}
		elemSize, sizeLen, err := readElementSize(br)
		if err != nil {
			return info, err
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		switch id {
		case TimecodeScaleID:
			val, _ := r.readUint(dataSize)
			info.TimecodeScale = int64(val)
		case DurationID:
			if dataSize == 4 || dataSize == 8 {
				if r.offset+dataSize > len(r.data) {
					r.offset += dataSize
					break
				}
				if dataSize == 4 {
					bits := binary.BigEndian.Uint32(r.data[r.offset:])
					info.Duration = float64(math.Float32frombits(bits))
				} else {
					bits := binary.BigEndian.Uint64(r.data[r.offset:])
					info.Duration = math.Float64frombits(bits)
				}
				r.offset += dataSize
			} else {
				r.offset += dataSize
			}
		default:
			r.offset += totalHeader + dataSize
		}
	}
	return info, nil
}

// ParseClusters parses all Cluster elements and returns their blocks.
func (r *EBMLReader) ParseClusters() ([]EBMLCluster, error) {
	var clusters []EBMLCluster

	for r.offset < len(r.data) {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br)
		if err != nil {
			break
		}
		elemSize, sizeLen, err := readElementSize(br)
		if err != nil {
			break
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)
		nextOffset := r.offset + totalHeader + dataSize

		if id == ClusterID {
			cluster, err := r.parseCluster(dataSize)
			if err != nil {
				return clusters, err
			}
			clusters = append(clusters, *cluster)
		} else {
			r.offset = nextOffset
		}
	}
	return clusters, nil
}

func (r *EBMLReader) parseCluster(size int) (*EBMLCluster, error) {
	end := r.offset + size
	cluster := &EBMLCluster{}

	for r.offset < end {
		br := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br)
		if err != nil {
			return cluster, err
		}
		elemSize, sizeLen, err := readElementSize(br)
		if err != nil {
			return cluster, err
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		switch id {
		case ClusterTimecodeID:
			val, _ := r.readUint(dataSize)
			cluster.Timecode = int64(val)
		case SimpleBlockID:
			sb, err := r.parseSimpleBlock(dataSize)
			if err != nil {
				return cluster, err
			}
			cluster.SimpleBlocks = append(cluster.SimpleBlocks, *sb)
		default:
			r.offset += totalHeader + dataSize
		}
	}
	return cluster, nil
}

// WebMParseResult holds the parsed WebM header information.
type WebMParseResult struct {
	Info   *EBMLInfo
	Tracks []EBMLAudioTrack
}

// ParseWebM parses a complete WebM file and returns the header info and audio tracks.
// It skips the EBML header, finds the Segment, and extracts Info + Tracks.
func ParseWebM(data []byte) (*WebMParseResult, error) {
	r := NewEBMLReader(data)

	// --- EBML header ---
	br := bytes.NewReader(r.data)
	ebmlID, _, err := readElementID(br)
	if err != nil {
		return nil, fmt.Errorf("read EBML ID: %w", err)
	}
	if ebmlID != EBMLID {
		return nil, fmt.Errorf("not EBML: expected 0x%X, got 0x%X", EBMLID, ebmlID)
	}
	ebmlSize, _, err := readElementSize(br)
	if err != nil {
		return nil, fmt.Errorf("read EBML size: %w", err)
	}
	// Skip past the entire EBML header
	r.offset = 4 + int(ebmlSize) + 8 // rough header size

	// --- Segment ---
	br2 := bytes.NewReader(r.data[r.offset:])
	segID, segIDLen, err := readElementID(br2)
	if err != nil {
		return nil, fmt.Errorf("read Segment ID: %w", err)
	}
	if segID != SegmentID {
		return nil, fmt.Errorf("expected Segment (0x%X), got 0x%X", SegmentID, segID)
	}
	segSize, segSizeLen, err := readElementSize(br2)
	if err != nil {
		return nil, fmt.Errorf("read Segment size: %w", err)
	}
	r.offset += segIDLen + segSizeLen
	segEnd := r.offset + int(segSize)

	// Scan Segment children for Info and Tracks
	result := &WebMParseResult{}
	for r.offset < segEnd {
		br3 := bytes.NewReader(r.data[r.offset:])
		id, idLen, err := readElementID(br3)
		if err != nil {
			break
		}
		elemSize, sizeLen, err := readElementSize(br3)
		if err != nil {
			break
		}
		totalHeader := idLen + sizeLen
		dataSize := int(elemSize)

		switch id {
		case InfoID:
			r.offset += totalHeader
			info, err := r.ParseSegmentInfo(dataSize)
			if err != nil {
				return nil, fmt.Errorf("parse Info: %w", err)
			}
			result.Info = info
		case TracksID:
			r.offset += totalHeader
			tracks, err := r.parseTracksBody(dataSize)
			if err != nil {
				return nil, fmt.Errorf("parse Tracks: %w", err)
			}
			result.Tracks = tracks
		default:
			r.offset += totalHeader + dataSize
		}

		// Early exit if we have both Info and Tracks
		if result.Info != nil && len(result.Tracks) > 0 {
			break
		}
	}

	if result.Info == nil {
		return nil, fmt.Errorf("Info element not found")
	}
	if len(result.Tracks) == 0 {
		return nil, fmt.Errorf("no audio tracks found")
	}
	return result, nil
}

func (r *EBMLReader) parseSimpleBlock(size int) (*EBMLSimpleBlock, error) {
	if r.offset+4 > len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}

	sb := &EBMLSimpleBlock{}
	start := r.offset

	// Track number (VINT)
	trackVal, trackLen, err := readVINT(bytes.NewReader(r.data[r.offset:]))
	if err != nil {
		return nil, err
	}
	sb.TrackNumber = int(trackVal)
	r.offset += trackLen

	// Timecode (int16, big-endian)
	if r.offset+2 > len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}
	sb.Timecode = int16(binary.BigEndian.Uint16(r.data[r.offset:]))
	r.offset += 2

	// Flags: 1 byte
	if r.offset >= len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}
	flags := r.data[r.offset]
	r.offset++
	sb.Keyframe = (flags&0x80 != 0)

	// Remaining bytes are frame data
	dataSize := size - (r.offset - start)
	if dataSize < 0 {
		return nil, fmt.Errorf("invalid simpleblock size")
	}
	if r.offset+dataSize > len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}
	sb.Data = make([]byte, dataSize)
	copy(sb.Data, r.data[r.offset:r.offset+dataSize])
	r.offset += dataSize

	return sb, nil
}
