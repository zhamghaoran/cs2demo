package storage

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"testing"
)

func TestSaveDemoContentExtractsDemoFromZipWithoutCentralDirectory(t *testing.T) {
	want := []byte("HL2DEMO\x00demo payload")
	var compressed bytes.Buffer
	zw, err := flate.NewWriter(&compressed, flate.DefaultCompression)
	if err != nil {
		t.Fatalf("new flate writer: %v", err)
	}
	if _, err := zw.Write(want); err != nil {
		t.Fatalf("write compressed payload: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close compressed payload: %v", err)
	}

	name := []byte("match.dem")
	var zipLike bytes.Buffer
	header := make([]byte, 30)
	binary.LittleEndian.PutUint32(header[0:4], 0x04034b50)
	binary.LittleEndian.PutUint16(header[4:6], 20)
	binary.LittleEndian.PutUint16(header[8:10], 8)
	binary.LittleEndian.PutUint32(header[18:22], uint32(compressed.Len()))
	binary.LittleEndian.PutUint32(header[22:26], uint32(len(want)))
	binary.LittleEndian.PutUint16(header[26:28], uint16(len(name)))
	zipLike.Write(header)
	zipLike.Write(name)
	zipLike.Write(compressed.Bytes())

	var got bytes.Buffer
	if err := saveDemoContent("match.zip", &zipLike, &got); err != nil {
		t.Fatalf("save demo content: %v", err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("extracted payload mismatch: got %q, want %q", got.Bytes(), want)
	}
}

func TestSaveDemoContentPadsRecoverableZipTail(t *testing.T) {
	wantPrefix := []byte("HL2DEMO\x00demo payload")
	var compressed bytes.Buffer
	zw, err := flate.NewWriter(&compressed, flate.DefaultCompression)
	if err != nil {
		t.Fatalf("new flate writer: %v", err)
	}
	if _, err := zw.Write(wantPrefix); err != nil {
		t.Fatalf("write compressed payload: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close compressed payload: %v", err)
	}

	name := []byte("match.dem")
	var zipLike bytes.Buffer
	header := make([]byte, 30)
	binary.LittleEndian.PutUint32(header[0:4], 0x04034b50)
	binary.LittleEndian.PutUint16(header[4:6], 20)
	binary.LittleEndian.PutUint16(header[8:10], 8)
	truncated := compressed.Bytes()[:compressed.Len()-1]
	binary.LittleEndian.PutUint32(header[18:22], uint32(compressed.Len()))
	binary.LittleEndian.PutUint32(header[22:26], uint32(len(wantPrefix)+4))
	binary.LittleEndian.PutUint16(header[26:28], uint16(len(name)))
	zipLike.Write(header)
	zipLike.Write(name)
	zipLike.Write(truncated)

	var got bytes.Buffer
	if err := saveDemoContent("match.zip", &zipLike, &got); err != nil {
		t.Fatalf("save demo content: %v", err)
	}
	if !bytes.Equal(got.Bytes()[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("extracted payload prefix mismatch: got %q, want %q", got.Bytes()[:len(wantPrefix)], wantPrefix)
	}
	if tail := got.Bytes()[len(wantPrefix):]; !bytes.Equal(tail, make([]byte, 4)) {
		t.Fatalf("expected zero-padded tail, got %q", tail)
	}
}

func TestSaveDemoContentCopiesPlainDemo(t *testing.T) {
	want := []byte("HL2DEMO\x00plain payload")
	var got bytes.Buffer
	if err := saveDemoContent("match.dem", bytes.NewReader(want), &got); err != nil {
		t.Fatalf("save demo content: %v", err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("copied payload mismatch: got %q, want %q", got.Bytes(), want)
	}
}
