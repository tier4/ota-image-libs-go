package artifact

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"testing"
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
		t.Error(err)
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
		t.Error(err)
	}
	defer func() {
		if err := b.Close(); err != nil {
			t.Logf("failed to close test file: %v", err)
		}
	}()

	_, err = processTestFile(b)
	// Expect to hit check sum error
	if err != ErrChecksum {
		t.Errorf("expected to get %v, but get %v", io.ErrUnexpectedEOF, err)
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
		t.Error(err)
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
