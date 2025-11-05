package artifact

import "strings"

// OTA image artifact doesn't compress ZIP file entry
const (
	Store uint16 = 0 // no compression
)

// copied from std zip lib
const (
	fileHeaderSignature      = 0x04034b50
	directoryHeaderSignature = 0x02014b50
	fileHeaderLen            = 30 // + filename + extra

	// Extra header IDs.
	//
	// We only care about zip64 extension.
	zip64ExtraID = 0x0001 // Zip64 extended information
)

// LocalFileHeader is a subset of the ZIP's local file header.
type LocalFileHeader struct {
	Name string // full filename of the entry

	Size  uint64 // uncompressed size of the file, as no compression is used in OTA image artifact
	CRC32 uint32 // CRC32 checksum of this file, caller may use this checksum to verify the read file
}

// IsDir tells if a file corresponds to a directory or not.
// Following ZIP convention, file with filename ends with `/` is considered a directory.
func (hdr *LocalFileHeader) IsDir() bool {
	return strings.HasSuffix(hdr.Name, "/")
}
