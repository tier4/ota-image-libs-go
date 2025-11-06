package artifact

import (
	"archive/zip"
	"errors"
	"hash"
	"hash/crc32"
	"io"
)

var (
	ErrInvalidOTAImageArtifact = errors.New("ota image: not an OTA image")
	ErrReadWithoutNext         = errors.New("ota image: Read before Next")
	ErrOverlappedNext          = errors.New("ota image: Next before previous read finished")

	// Inherit from std zip lib
	ErrChecksum = zip.ErrChecksum
)

// A StreamReader streams file entries from an IO stream serving OTA image artifact.
type StreamReader struct {
	r    io.Reader
	curr *checksumFileStreamReader

	err error // a persist error holding the last error during streaming.
}

// NewReader returns a new [StreamReader] reading files from an IO stream serving
// OTA image artifact.
func NewReader(r io.Reader) *StreamReader {
	return &StreamReader{r: r}
}

// Next reads the next local file header and advances the IO stream to the data section
// of the next file.
//
// If we already hit error in previous read, will directly return without further reading.
func (zr *StreamReader) Next() (*LocalFileHeader, error) {
	if zr.err != nil {
		return nil, zr.err
	}

	// if previous read is not yet finished
	if curr := zr.curr; curr != nil && curr.readBytes != curr.size {
		return nil, ErrOverlappedNext
	}

	hdr, err := zr.next()
	if err != nil {
		zr.err = err
		return nil, err
	}
	return hdr, err
}

func (zr *StreamReader) next() (*LocalFileHeader, error) {
	hdr, err := zr.readLocalFileHeader()
	if err != nil {
		return nil, err
	}

	zr.curr = &checksumFileStreamReader{
		hdr: hdr, r: zr.r, size: hdr.Size, readBytes: 0, hash: crc32.NewIEEE(),
	}
	return hdr, nil
}

// readLocalFileHeader implements the actual logic of reading and parsing a ZIP local file header
// and advances the IO stream to the start of the data section.
func (zr *StreamReader) readLocalFileHeader() (*LocalFileHeader, error) {
	var buf [fileHeaderLen]byte
	r := zr.r

	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return nil, err
	}
	b := readBuf(buf[:])

	headerSig := b.uint32()
	// we have read all files and advanced to the central directory
	if headerSig == directoryHeaderSignature {
		return nil, io.EOF
	}

	if headerSig != fileHeaderSignature {
		return nil, ErrInvalidOTAImageArtifact
	}

	// read through header fields
	hdr := &LocalFileHeader{}
	b.uint16() // min version for extraction
	b.uint16() // general purpose flag
	compress_method := b.uint16()
	b.uint16() // modified time
	b.uint16() // modified date
	hdr.CRC32 = b.uint32()
	b.uint32()          // compressed size
	uSize := b.uint32() // uncompressed size
	filenameLen := int(b.uint16())
	extraLen := int(b.uint16())
	d := make([]byte, filenameLen+extraLen)
	if _, err := io.ReadFull(r, d); err != nil {
		return nil, err
	}
	hdr.Name = string(d[:filenameLen])
	extraField := d[filenameLen : filenameLen+extraLen]
	hdr.Size = uint64(uSize)

	// sanity check, OTA image artifact doesn't do compression
	if compress_method != Store {
		return nil, ErrInvalidOTAImageArtifact
	}

	// check extra fields
	needUSize := uSize == ^uint32(0)
	for extra := readBuf(extraField); len(extra) >= 4; {
		fieldTag := extra.uint16()
		fieldSize := int(extra.uint16())
		if len(extra) < fieldSize {
			break
		}
		fieldBuf := extra.sub(fieldSize)

		// we only care about zip64 extension
		switch fieldTag {
		case zip64ExtraID:
			if needUSize {
				needUSize = false
				if len(fieldBuf) < 8 {
					return nil, zip.ErrFormat
				}
				hdr.Size = fieldBuf.uint64()
			}
		}
	}

	// now the IO stream should be at the start of data section
	return hdr, nil
}

// Read read [len(b)] of data of the current file from the stream.
func (zr *StreamReader) Read(b []byte) (int, error) {
	if zr.err != nil {
		return 0, zr.err
	}
	if zr.curr == nil {
		return 0, ErrReadWithoutNext
	}

	n, err := zr.curr.Read(b)
	if err != nil && err != io.EOF {
		zr.err = err
	}
	return n, err
}

type checksumFileStreamReader struct {
	hdr       *LocalFileHeader
	r         io.Reader // underlying Reader
	size      uint64    // size of the file
	readBytes uint64    // bytes we already read

	hash hash.Hash32 // for CRC32

}

// Read reads data chunk to [b] from the underlaying IO stream,
// while doing CRC32 checksum calculation.
func (fr *checksumFileStreamReader) Read(b []byte) (n int, err error) {
	// check how many bytes we should read
	if fr.readBytes > fr.size {
		return 0, io.ErrUnexpectedEOF
	}
	nb := fr.size - fr.readBytes

	// if we get a super long byte array, cap the input array
	if uint64(len(b)) > nb {
		b = b[:nb]
	}

	if len(b) > 0 {
		n, err = fr.r.Read(b)
		fr.readBytes += uint64(n) // #nosec G115 : we will not read negative bytes
		fr.hash.Write(b[:n])

		// see how many bytes to read we left
		nb -= uint64(n) // #nosec G115
	}

	switch {
	case err == io.EOF && nb > 0:
		return n, io.ErrUnexpectedEOF
	case err == nil && nb == 0:
		// finish up reading this file, check CRC32
		if fr.hdr.CRC32 != 0 && fr.hash.Sum32() != fr.hdr.CRC32 {
			err = ErrChecksum
			return n, err
		}
		return n, io.EOF
	default:
		return n, err
	}
}
