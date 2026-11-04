package artifact

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"
)

type OTAImageArtifactTestFile struct {
	Name       string
	Size       uint64
	FilesCount int // total number of files in the artifact
}

func openTestFile(fName string) (fs.File, error) {
	b, err := os.Open(fmt.Sprintf("./testdata/%s", fName))
	return b, err
}

func processTestFile(b io.Reader) (int, error) {
	r := NewReader(b)
	buf := make([]byte, 1024*1024) // 1MiB
	var i int
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			fmt.Printf("finish up reading! Total %d files read\n", i)
			return i, nil
		}
		if err != nil {
			return 0, err
		}

		if !hdr.IsDir() {
			i += 1
		}

		// reading the data of the file
		for {
			_, err := r.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				return 0, err
			}
		}
	}
}

var normalArtifact = OTAImageArtifactTestFile{
	Name:       "ota_image.zip",
	Size:       25586346,
	FilesCount: 1103, // exclude directories
}

func TestReadOTAImageArtifact(t *testing.T) {
	b, err := openTestFile(normalArtifact.Name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := b.Close(); err != nil {
			t.Logf("failed to close test file: %v", err)
		}
	}()
	n, err := processTestFile(b)
	if err != nil {
		t.Error(err)
	}

	// confirm that all files are read
	if n != normalArtifact.FilesCount {
		t.Errorf("files count mismatched")
	}
}

var otaImageWith6Gblob = OTAImageArtifactTestFile{
	Name:       "ota_image_6g_blob.zip",
	Size:       6369977451,
	FilesCount: 2406, // exclude directories
}

func TestReadOTAImageArtifactWith6Gblob(t *testing.T) {
	b, err := openTestFile(otaImageWith6Gblob.Name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := b.Close(); err != nil {
			t.Logf("failed to close test file: %v", err)
		}
	}()
	n, err := processTestFile(b)
	if err != nil {
		t.Error(err)
	}

	// confirm that all files are read
	if n != otaImageWith6Gblob.FilesCount {
		t.Errorf("files count mismatched")
	}
}

var truncatedArtifact = OTAImageArtifactTestFile{
	Name: "ota_image_truncated.zip",
	Size: 10485760,
}

func TestReadTruncatedOTAImageA(t *testing.T) {
	b, err := openTestFile(truncatedArtifact.Name)
	if err != nil {
		t.Fatalf("failed to open test files")
	}
	defer func() {
		if err := b.Close(); err != nil {
			t.Logf("failed to close test file: %v", err)
		}
	}()

	_, err = processTestFile(b)
	// Expect to see unexpected EOF
	if err != io.ErrUnexpectedEOF {
		t.Errorf("expected to get %v, but get %v", io.ErrUnexpectedEOF, err)
	}
}

// some files' data section is damaged/altered
var damangedOTAImageArtifact = OTAImageArtifactTestFile{
	Name: "ota_image_damaged.zip",
	Size: 25586346,
}

func TestReadDamagedOTAImageA(t *testing.T) {
	b, err := openTestFile(damangedOTAImageArtifact.Name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := b.Close(); err != nil {
			t.Logf("failed to close test file: %v", err)
		}
	}()

	_, err = processTestFile(b)
	// Expect to hit check sum error
	if err != ErrChecksum {
		t.Errorf("expected to get %v, but get %v", ErrChecksum, err)
	}
}

// some files' header section is damaged/altered
var headerDamangedOTAImageArtifact = OTAImageArtifactTestFile{
	Name: "ota_image_header_damaged.zip",
	Size: 25586346,
}

func TestReadHeaderDamagedOTAImageA(t *testing.T) {
	b, err := openTestFile(headerDamangedOTAImageArtifact.Name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := b.Close(); err != nil {
			t.Logf("failed to close test file: %v", err)
		}
	}()

	_, err = processTestFile(b)
	// Expect to hit invalid OTA image error
	if err != ErrInvalidOTAImageArtifact {
		t.Errorf("expected to get %v, but get %v", ErrInvalidOTAImageArtifact, err)
	}
}

// TestReadOTAImageArtifactMatchesCentralDirectory checks every streamed entry against
// the central directory parsed by archive/zip.
func TestReadOTAImageArtifactMatchesCentralDirectory(t *testing.T) {
	for _, tf := range []OTAImageArtifactTestFile{normalArtifact, otaImageWith6Gblob} {
		t.Run(tf.Name, func(t *testing.T) {
			zf, err := zip.OpenReader(filepath.Join("testdata", tf.Name))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := zf.Close(); err != nil {
					t.Logf("failed to close test file: %v", err)
				}
			}()
			b, err := openTestFile(tf.Name)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := b.Close(); err != nil {
					t.Logf("failed to close test file: %v", err)
				}
			}()

			r := NewReader(b)
			buf := make([]byte, 1024*1024) // 1MiB
			for i, want := range zf.File {
				hdr, err := r.Next()
				if err != nil {
					t.Fatalf("entry %d (%s): %v", i, want.Name, err)
				}
				if hdr.Name != want.Name || hdr.Size != want.UncompressedSize64 || hdr.CRC32 != want.CRC32 {
					t.Fatalf("entry %d: expected {%s %d %08x}, but get {%s %d %08x}",
						i, want.Name, want.UncompressedSize64, want.CRC32, hdr.Name, hdr.Size, hdr.CRC32)
				}

				h := crc32.NewIEEE()
				n, err := io.CopyBuffer(h, r, buf)
				if err != nil {
					t.Fatalf("entry %d (%s): %v", i, want.Name, err)
				}
				// #nosec G115: io.CopyBuffer returns non-negative count
				if uint64(n) != want.UncompressedSize64 || h.Sum32() != want.CRC32 {
					t.Fatalf("entry %d (%s): expected %d bytes with CRC32 %08x, but get %d bytes with CRC32 %08x",
						i, want.Name, want.UncompressedSize64, want.CRC32, n, h.Sum32())
				}
			}
			if _, err := r.Next(); err != io.EOF {
				t.Errorf("expected to get %v after the last entry, but get %v", io.EOF, err)
			}
		})
	}
}

// testEntry is a ZIP local file header followed by the file data,
// used to build OTA image artifacts in memory.
type testEntry struct {
	name   string
	data   []byte
	method uint16
	crc32  uint32
	size   uint32 // both compressed and uncompressed size
	extra  []byte
}

// newTestEntry returns a valid entry storing data.
func newTestEntry(name string, data []byte) testEntry {
	return testEntry{
		name:   name,
		data:   data,
		method: Store,
		crc32:  crc32.ChecksumIEEE(data),
		size:   uint32(len(data)), // #nosec G115: test data is small
	}
}

func (e testEntry) bytes() []byte {
	le := binary.LittleEndian
	b := le.AppendUint32(nil, fileHeaderSignature)
	b = le.AppendUint16(b, 20) // min version for extraction
	b = le.AppendUint16(b, 0)  // general purpose flag
	b = le.AppendUint16(b, e.method)
	b = le.AppendUint16(b, 0) // modified time
	b = le.AppendUint16(b, 0) // modified date
	b = le.AppendUint32(b, e.crc32)
	b = le.AppendUint32(b, e.size)               // compressed size
	b = le.AppendUint32(b, e.size)               // uncompressed size
	b = le.AppendUint16(b, uint16(len(e.name)))  // #nosec G115: test filename is short
	b = le.AppendUint16(b, uint16(len(e.extra))) // #nosec G115: test extra field is short
	b = append(b, e.name...)
	b = append(b, e.extra...)
	return append(b, e.data...)
}

// directoryHeaderLen is the length of a central directory file header without
// the variable length fields.
const directoryHeaderLen = 46

// buildTestArtifact concatenates entries and terminates them with a zero-filled
// central directory file header, where StreamReader stops reading.
func buildTestArtifact(entries ...testEntry) []byte {
	var buf bytes.Buffer
	for _, e := range entries {
		buf.Write(e.bytes())
	}
	cd := make([]byte, directoryHeaderLen)
	binary.LittleEndian.PutUint32(cd, directoryHeaderSignature)
	buf.Write(cd)
	return buf.Bytes()
}

// zip64Extra returns a zip64 extended information extra field
// holding size as both the uncompressed and compressed size.
func zip64Extra(size uint64) []byte {
	le := binary.LittleEndian
	b := le.AppendUint16(nil, zip64ExtraID)
	b = le.AppendUint16(b, 16)
	b = le.AppendUint64(b, size) // uncompressed size
	return le.AppendUint64(b, size)
}

type readEntry struct {
	hdr  *LocalFileHeader
	data []byte
}

// readAllEntries streams entries from r until the end of the artifact or the first error.
func readAllEntries(r io.Reader) ([]readEntry, error) {
	zr := NewReader(r)
	var entries []readEntry
	for {
		hdr, err := zr.Next()
		if err == io.EOF {
			return entries, nil
		}
		if err != nil {
			return entries, err
		}
		data, err := io.ReadAll(zr)
		if err != nil {
			return entries, err
		}
		entries = append(entries, readEntry{hdr: hdr, data: data})
	}
}

// readerWrappers simulate IO streams returning data in different chunks.
var readerWrappers = []struct {
	name string
	wrap func(io.Reader) io.Reader
}{
	{"plain", func(r io.Reader) io.Reader { return r }},
	{"one byte per read", iotest.OneByteReader},
	{"half per read", iotest.HalfReader},
	{"EOF with last data", iotest.DataErrReader},
}

var testEntries = []testEntry{
	newTestEntry("dir/", nil),
	newTestEntry("dir/empty", nil),
	newTestEntry("dir/file", []byte("hello, OTA image")),
	newTestEntry("blob", bytes.Repeat([]byte("0123456789abcdef"), 64)),
}

func TestReadArtifactFromVariousReaders(t *testing.T) {
	artifact := buildTestArtifact(testEntries...)
	for _, rw := range readerWrappers {
		t.Run(rw.name, func(t *testing.T) {
			got, err := readAllEntries(rw.wrap(bytes.NewReader(artifact)))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(testEntries) {
				t.Fatalf("expected %d entries, but get %d", len(testEntries), len(got))
			}
			for i, want := range testEntries {
				hdr := got[i].hdr
				if hdr.Name != want.name || hdr.Size != uint64(want.size) || hdr.CRC32 != want.crc32 {
					t.Errorf("entry %d: expected {%s %d %08x}, but get {%s %d %08x}",
						i, want.name, want.size, want.crc32, hdr.Name, hdr.Size, hdr.CRC32)
				}
				if !bytes.Equal(got[i].data, want.data) {
					t.Errorf("entry %d (%s): data mismatched", i, want.name)
				}
			}
		})
	}
}

// An OTA image artifact always ends with the central directory, a stream cut at any
// offset before it is truncated, including cuts at the boundary of entries.
func TestReadArtifactTruncatedAtEveryOffset(t *testing.T) {
	artifact := buildTestArtifact(testEntries...)
	// StreamReader stops once the fixed part of the first central directory header is read.
	end := len(artifact) - directoryHeaderLen + fileHeaderLen
	for _, rw := range readerWrappers {
		t.Run(rw.name, func(t *testing.T) {
			for n := 0; n < end; n++ {
				_, err := readAllEntries(rw.wrap(bytes.NewReader(artifact[:n])))
				if err != io.ErrUnexpectedEOF {
					t.Errorf("cut at %d/%d: expected to get %v, but get %v", n, len(artifact), io.ErrUnexpectedEOF, err)
				}
			}
		})
	}
}

func TestReadChecksumMismatch(t *testing.T) {
	e := newTestEntry("file", []byte("data"))
	e.crc32 = ^e.crc32
	cases := []struct {
		name     string
		artifact []byte
	}{
		{"complete artifact", buildTestArtifact(e)},
		// the underlying stream hits EOF right at the end of the file data
		{"cut after file data", e.bytes()},
	}
	for _, c := range cases {
		for _, rw := range readerWrappers {
			t.Run(c.name+"/"+rw.name, func(t *testing.T) {
				r := NewReader(rw.wrap(bytes.NewReader(c.artifact)))
				if _, err := r.Next(); err != nil {
					t.Fatal(err)
				}
				if _, err := io.ReadAll(r); err != ErrChecksum {
					t.Errorf("expected to get %v, but get %v", ErrChecksum, err)
				}
			})
		}
	}
}

func TestReadAfterError(t *testing.T) {
	broken := newTestEntry("broken", []byte("data"))
	broken.crc32 = ^broken.crc32
	r := NewReader(bytes.NewReader(buildTestArtifact(broken, newTestEntry("file", []byte("data")))))
	if _, err := r.Next(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(r); err != ErrChecksum {
		t.Fatalf("expected to get %v, but get %v", ErrChecksum, err)
	}

	// the error persists, no further reading from the stream
	if _, err := r.Next(); err != ErrChecksum {
		t.Errorf("Next: expected to get %v, but get %v", ErrChecksum, err)
	}
	if _, err := r.Read(make([]byte, 1)); err != ErrChecksum {
		t.Errorf("Read: expected to get %v, but get %v", ErrChecksum, err)
	}
}

func TestReadWithoutNext(t *testing.T) {
	r := NewReader(bytes.NewReader(buildTestArtifact(newTestEntry("file", []byte("data")))))
	if _, err := r.Read(make([]byte, 1)); err != ErrReadWithoutNext {
		t.Errorf("expected to get %v, but get %v", ErrReadWithoutNext, err)
	}
}

func TestNextBeforeReadFinished(t *testing.T) {
	r := NewReader(bytes.NewReader(buildTestArtifact(
		newTestEntry("first", []byte("0123456789")),
		newTestEntry("second", []byte("abc")),
	)))
	if _, err := r.Next(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(r, make([]byte, 4)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Next(); err != ErrOverlappedNext {
		t.Fatalf("expected to get %v, but get %v", ErrOverlappedNext, err)
	}

	// the error doesn't persist, caller can finish reading the file and move on
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(rest) != "456789" {
		t.Errorf("expected to read %q, but get %q", "456789", rest)
	}
	hdr, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Name != "second" {
		t.Errorf("expected to get %q, but get %q", "second", hdr.Name)
	}
}

func TestReadInvalidLocalFileHeader(t *testing.T) {
	compressed := newTestEntry("file", []byte("data"))
	compressed.method = 8 // deflate

	shortZip64 := newTestEntry("file", []byte("data"))
	shortZip64.size = ^uint32(0)
	// zip64 extra field with only 4 bytes of data
	shortZip64.extra = []byte{0x01, 0x00, 0x04, 0x00, 0x04, 0x00, 0x00, 0x00}

	cases := []struct {
		name     string
		artifact []byte
		err      error
	}{
		{"bad signature", bytes.Repeat([]byte{0xff}, fileHeaderLen), ErrInvalidOTAImageArtifact},
		{"compressed entry", buildTestArtifact(compressed), ErrInvalidOTAImageArtifact},
		{"short zip64 extra field", buildTestArtifact(shortZip64), zip.ErrFormat},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewReader(bytes.NewReader(c.artifact)).Next(); err != c.err {
				t.Errorf("expected to get %v, but get %v", c.err, err)
			}
		})
	}
}

func TestReadExtraField(t *testing.T) {
	const data = "zip64 entry"
	zip64 := newTestEntry("zip64", []byte(data))
	zip64.size = ^uint32(0) // the actual size is in the zip64 extra field
	// an extended timestamp extra field before the zip64 one
	zip64.extra = append([]byte{0x55, 0x54, 0x05, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}, zip64Extra(uint64(len(data)))...)

	malformed := newTestEntry("malformed", []byte(data))
	// the extra field declares more data than it has, and is ignored
	malformed.extra = []byte{0xfe, 0xca, 0xff, 0x00, 0x00}

	got, err := readAllEntries(bytes.NewReader(buildTestArtifact(zip64, malformed)))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, but get %d", len(got))
	}
	for _, e := range got {
		if e.hdr.Size != uint64(len(data)) || string(e.data) != data {
			t.Errorf("%s: expected %d bytes %q, but get %d bytes %q", e.hdr.Name, len(data), data, e.hdr.Size, e.data)
		}
	}
}
