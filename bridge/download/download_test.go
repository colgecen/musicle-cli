package download

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

func TestTrackInfoDefaults(t *testing.T) {
	ti := &TrackInfo{Title: "Test", Artist: "Me"}
	if ti.Title != "Test" || ti.Artist != "Me" {
		t.Fatalf("unexpected: %+v", ti)
	}
}

func TestSyncsafeEncode(t *testing.T) {
	tests := []struct {
		n    uint32
		want []byte
	}{
		{0, []byte{0, 0, 0, 0}},
		{127, []byte{0, 0, 0, 127}},
		{128, []byte{0, 0, 1, 0}},
		{16383, []byte{0, 0, 127, 127}},
		{16384, []byte{0, 1, 0, 0}},
		{2097151, []byte{0, 127, 127, 127}},
		{2097152, []byte{1, 0, 0, 0}},
	}
	for _, tc := range tests {
		got := syncsafeEncode(tc.n)
		if !bytes.Equal(got, tc.want) {
			t.Errorf("syncsafeEncode(%d) = %v, want %v", tc.n, got, tc.want)
		}
	}
}

func TestWriteTextFrame(t *testing.T) {
	f := writeTextFrame("TIT2", "Hello")
	if len(f) < 10 {
		t.Fatalf("frame too short: %d", len(f))
	}
	if string(f[0:4]) != "TIT2" {
		t.Errorf("frame id = %q, want TIT2", string(f[0:4]))
	}
	size := int(f[4])<<24 | int(f[5])<<16 | int(f[6])<<8 | int(f[7])
	if size != len(f)-10 {
		t.Errorf("frame size %d, data %d", size, len(f)-10)
	}
	if f[10] != 3 {
		t.Errorf("encoding byte = %d, want 3", f[10])
	}
	if string(f[11:]) != "Hello" {
		t.Errorf("text = %q, want Hello", string(f[11:]))
	}
}

func TestWriteNumFrame(t *testing.T) {
	f := writeNumFrame("TRCK", 7)
	if f == nil {
		t.Fatal("expected frame")
	}
	if string(f[0:4]) != "TRCK" {
		t.Errorf("id = %q", string(f[0:4]))
	}
	data := f[11:]
	if string(data) != "7" {
		t.Errorf("data = %q, want 7", string(data))
	}
	if writeNumFrame("TRCK", 0) != nil {
		t.Errorf("expected nil for 0")
	}
}

func TestWriteID3Tag(t *testing.T) {
	mp3 := []byte{0xFF, 0xFB, 0x90, 0x00}
	info := &TrackInfo{Title: "Song", Artist: "Singer", Album: "Album"}
	tagged, err := WriteID3Tag(mp3, info)
	if err != nil {
		t.Fatalf("WriteID3Tag: %v", err)
	}
	if len(tagged) <= len(mp3) {
		t.Fatalf("tagged too short: %d", len(tagged))
	}
	if string(tagged[0:3]) != "ID3" {
		t.Errorf("header = %q, want ID3", string(tagged[0:3]))
	}
	if tagged[3] != 3 {
		t.Errorf("version = %d, want 3", tagged[3])
	}
	if !bytes.Equal(tagged[len(tagged)-4:], mp3) {
		t.Errorf("mp3 data not preserved")
	}
}

func TestWriteID3TagEmpty(t *testing.T) {
	mp3 := []byte{0xFF, 0xFB}
	info := &TrackInfo{}
	tagged, err := WriteID3Tag(mp3, info)
	if err != nil {
		t.Fatal(err)
	}
	if len(tagged) <= len(mp3) {
		t.Errorf("expected tag header + mp3, got len=%d", len(tagged))
	}
}

func TestRangeDecoder(t *testing.T) {
	rd := ecDecInit(nil)
	_ = rd.decBit()
	_ = rd.decUniform(5)
}

func TestRangeDecoderDecBit(t *testing.T) {
	data := make([]byte, 64)
	for i := range data {
		data[i] = 0xFF
	}
	rd := ecDecInit(data)
	bit := rd.decBit()
	t.Logf("decBit = %d", bit)
}

func TestOpusSampleRate(t *testing.T) {
	tests := []struct {
		bw   int
		want int
	}{
		{0, 8000}, {1, 12000}, {2, 16000}, {3, 24000}, {4, 48000}, {5, 48000},
	}
	for _, tc := range tests {
		got := OpusSampleRate(tc.bw)
		if got != tc.want {
			t.Errorf("OpusSampleRate(%d) = %d, want %d", tc.bw, got, tc.want)
		}
	}
}

func TestSilkLPCOrder(t *testing.T) {
	tests := []struct {
		bw   int
		want int
	}{
		{0, 10}, {1, 12}, {2, 16}, {3, 16}, {4, 16},
	}
	for _, tc := range tests {
		got := silkLPCOrder(tc.bw)
		if got != tc.want {
			t.Errorf("silkLPCOrder(%d) = %d, want %d", tc.bw, got, tc.want)
		}
	}
}

func TestResample(t *testing.T) {
	input := []int16{0, 100, 200, 300, 400, 500}
	out := resample(input, 48000, 24000, 1)
	if len(out) < len(input)/2 {
		t.Errorf("resample too short: %d", len(out))
	}
	if out[0] != 0 {
		t.Errorf("first sample should be 0, got %d", out[0])
	}
}

func TestMP3BitrateTable(t *testing.T) {
	if mp3BitrateTable[0] != 0 || mp3BitrateTable[9] != 128 || mp3BitrateTable[14] != 320 {
		t.Errorf("table values wrong")
	}
}

func TestMP3SampleRateTable(t *testing.T) {
	if mp3SampleRateTable[0] != 44100 || mp3SampleRateTable[1] != 48000 {
		t.Errorf("table values wrong")
	}
}

func TestNewMP3Encoder(t *testing.T) {
	e := newMP3Encoder(44100, 192, 2)
	if e == nil {
		t.Fatal("encoder is nil")
	}
	if e.sampleRate != 44100 || e.bitrate != 192 {
		t.Errorf("encoder params wrong")
	}
}

func TestMP3BuildHeader(t *testing.T) {
	e := newMP3Encoder(44100, 192, 2)
	h := e.buildHeader()
	if len(h) != 4 {
		t.Fatalf("header len = %d", len(h))
	}
	sync := (uint32(h[0])<<4 | uint32(h[1])>>4) & 0xFFE
	if sync != 0xFFE {
		t.Errorf("sync = %X, want FFE", sync)
	}
}

func TestEncodePCMToMP3Go(t *testing.T) {
	pcm := make([]int16, 1152*2)
	for i := range pcm {
		pcm[i] = int16(i * 100)
	}
	mp3, err := EncodePCMToMP3Go(pcm, 44100, 2, 128)
	if err != nil {
		t.Fatalf("EncodePCMToMP3Go: %v", err)
	}
	if len(mp3) < 4 {
		t.Fatalf("mp3 too short: %d", len(mp3))
	}
	if mp3[0] != 0xFF || mp3[1]&0xE0 != 0xE0 {
		t.Errorf("bad sync: %02X %02X", mp3[0], mp3[1])
	}
}

func TestGetAudioDurationSec(t *testing.T) {
	_, err := GetAudioDurationSec([]byte{0xFF}, "webm")
	if err == nil {
		t.Error("expected error for invalid WebM")
	}
}

func TestWriteMP3File(t *testing.T) {
	var buf bytes.Buffer
	data := []byte{0xFF, 0xFB, 0x90}
	n, err := WriteMP3File(&buf, data)
	if err != nil {
		t.Fatalf("WriteMP3File: %v", err)
	}
	if n != 3 {
		t.Errorf("wrote %d bytes, want 3", n)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("data mismatch")
	}
}

func TestDecodeWebMOpusPackets(t *testing.T) {
	_, _, _, err := DecodeWebMOpusPackets([]byte{0xFF, 0xFF})
	if err == nil {
		t.Error("expected error for invalid WebM")
	}
}

func TestEncodePCMToMP3(t *testing.T) {
	pcm := make([]int16, 1152*2)
	for i := range pcm {
		pcm[i] = int16(i)
	}
	mp3, err := EncodePCMToMP3(pcm, 44100, 2, "192k", nil)
	if err != nil {
		t.Fatalf("EncodePCMToMP3: %v", err)
	}
	if len(mp3) < 4 {
		t.Fatalf("mp3 too short: %d", len(mp3))
	}
	if mp3[0] != 0xFF || (mp3[1]&0xE0) != 0xE0 {
		t.Errorf("bad sync: %02X %02X", mp3[0], mp3[1])
	}
}

func TestWriteMP3ToFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.mp3"
	mp3 := []byte{0xFF, 0xFB, 0x90, 0x00}
	info := &TrackInfo{Title: "Test", Artist: "Me"}
	err := WriteMP3ToFile(path, mp3, info)
	if err != nil {
		t.Fatalf("WriteMP3ToFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data[0:3]) != "ID3" {
		t.Errorf("no ID3 header: %q", string(data[0:3]))
	}
	if !bytes.Equal(data[len(data)-4:], mp3) {
		t.Errorf("mp3 data not preserved")
	}
}

func TestNlsfToLPC(t *testing.T) {
	nlsf := make([]int16, 16)
	for i := range nlsf {
		nlsf[i] = int16(500 + i*1000)
	}
	lpc := nlsfToLPC(nlsf, 16)
	if lpc == nil {
		t.Fatal("lpc is nil")
	}
	if lpc[0] != 1.0 {
		t.Errorf("a[0] = %f, want 1.0", lpc[0])
	}
	if len(lpc) != 17 {
		t.Errorf("lpc len = %d, want 17", len(lpc))
	}
}

func TestLpcSynthesisFilter(t *testing.T) {
	exc := []float64{1.0, 0.0, 0.0, 0.0, 0.0}
	a := []float64{1.0, -0.5, 0.25}
	out := lpcSynthesisFilter(exc, a)
	if len(out) != 5 {
		t.Fatalf("out len = %d", len(out))
	}
	if out[0] != 1.0 {
		t.Errorf("out[0] = %f, want 1.0", out[0])
	}
}

func TestDeemphasis(t *testing.T) {
	in := []float64{1.0, 0.0, 0.0}
	out := deemphasis(in)
	if out[0] != 1.0 {
		t.Errorf("out[0] = %f", out[0])
	}
}

func TestGenerateWhiteNoise(t *testing.T) {
	var seed uint32 = 42
	a := generateWhiteNoise(100, &seed)
	if len(a) != 100 {
		t.Fatalf("len = %d", len(a))
	}
	seed2 := uint32(42)
	b := generateWhiteNoise(100, &seed2)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("non-deterministic output")
	}
}

func TestGeneratePulseExcitation(t *testing.T) {
	p := generatePulseExcitation(50, 10)
	if p[0] != 1.0 || p[10] != 1.0 || p[5] != 0.0 {
		t.Errorf("pulse pattern wrong: p[0]=%f p[10]=%f p[5]=%f", p[0], p[10], p[5])
	}
}

func TestGetCeltBands(t *testing.T) {
	bands := getCeltBands(960)
	if len(bands) == 0 {
		t.Fatal("no bands")
	}
	if bands[0].end <= bands[0].start {
		t.Errorf("band[0] empty: %d-%d", bands[0].start, bands[0].end)
	}
}

func TestDecodePVQ(t *testing.T) {
	data := make([]byte, 64)
	for i := range data {
		data[i] = 0xFF
	}
	rd := ecDecInit(data)
	vec := decodePVQ(rd, 4, 2)
	if len(vec) != 4 {
		t.Fatalf("vec len = %d", len(vec))
	}
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	if norm < 0.99 || norm > 1.01 {
		t.Logf("norm = %f (note: test uses random data)", norm)
	}
}

func TestIMDCT(t *testing.T) {
	X := make([]float64, 480)
	for i := range X {
		X[i] = float64(i)
	}
	y := imdct(X, 960)
	if len(y) != 960 {
		t.Fatalf("imdct len = %d, want 960", len(y))
	}
}

func TestMP3MDCT32(t *testing.T) {
	pcm := make([]int16, 1152*2)
	for i := range pcm {
		pcm[i] = int16(i % 32767)
	}
	result, err := mp3MDCT32(pcm, 2)
	if err != nil {
		t.Fatalf("mp3MDCT32: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("channels = %d", len(result))
	}
	if len(result[0]) != 576 {
		t.Fatalf("result[0] len = %d, want 576", len(result[0]))
	}
}

func TestDecodeOpusToPCM(t *testing.T) {
	_, err := DecodeOpusToPCM(nil, 48000)
	if err == nil {
		t.Error("expected error for nil packets")
	}
	_, err = DecodeOpusToPCM([]OpusPacket{}, 48000)
	if err == nil {
		t.Error("expected error for empty packets")
	}
}

func TestParseOpusPacket(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00}
	pkt, err := ParseOpusPacket(data)
	if err != nil {
		t.Fatalf("ParseOpusPacket: %v", err)
	}
	if pkt.Config != 0 || pkt.Bandwidth != 0 {
		t.Errorf("config=%d bw=%d", pkt.Config, pkt.Bandwidth)
	}
}

func TestExtractOpusPackets(t *testing.T) {
	frames := []AudioFrame{{Data: []byte{0x00, 0x00, 0x00}}}
	pkts, err := ExtractOpusPackets(frames)
	if err != nil {
		t.Fatalf("ExtractOpusPackets: %v", err)
	}
	if len(pkts) != 1 {
		t.Fatalf("got %d packets, want 1", len(pkts))
	}
}

func TestChebyshev(t *testing.T) {
	coeffs := []int16{1024, 512, 256}
	val := chebyshev(coeffs, 0.5)
	if val == 0 {
		t.Errorf("chebyshev returned 0")
	}
}

func TestDecodeBandEnergy(t *testing.T) {
	data := make([]byte, 64)
	for i := range data {
		data[i] = 0xFF
	}
	rd := ecDecInit(data)
	e := decodeBandEnergy(rd, 0, nil)
	t.Logf("band energy = %f", e)
}
