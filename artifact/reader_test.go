package artifact

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"testing"
)

//go:embed testdata/*
var testFS embed.FS

type OTAImageArtifactTestFile struct {
	Name       string
	Size       uint64
	FilesCount uint64 // total number of files in the artifact
}

var TestFiles = []OTAImageArtifactTestFile{
	{
		Name:       "ota_image.zip",
		Size:       25586346,
		FilesCount: 1103, // exclude directories
	},
	{
		Name: "ota_image_truncated.zip",
		Size: 10485760,
	},
	{
		Name: "ota_image_damaged.zip",
		Size: 25586346,
	},
}

func openTestFile(fName string) (fs.File, error) {
	b, err := testFS.Open(fName)
	return b, err
}

func processTestFile(b io.Reader) error {
	r := NewReader(b)
	buf := make([]byte, 1024*1024) // 1MiB
	var i int
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			fmt.Printf("finish up reading! Total %d files read\n", i)
			break
		}
		if !hdr.IsDir() {
			i += 1
		}

		if err != nil {
			fmt.Printf("failed during streaming: %s", err)
			return err
		}

		// reading the data of the file
		for {
			_, err := r.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
		}
	}

	// confirm that all files are read
	if i != int(normalArtifact.FilesCount) {
		return errors.New("files count mismatched")
	}
	return nil
}

var normalArtifact = OTAImageArtifactTestFile{
	Name:       "ota_image.zip",
	Size:       25586346,
	FilesCount: 1103, // exclude directories
}

func TestReadOTAImageArtifact(t *testing.T) {
	testF := fmt.Sprintf("testdata/%s", normalArtifact.Name)
	b, err := openTestFile(testF)
	if err != nil {
		t.Error(err)
	}
	defer b.Close()

	if err := processTestFile(b); err != nil {
		t.Error(err)
	}
}

var truncatedArtifact = OTAImageArtifactTestFile{
	Name: "ota_image_truncated.zip",
	Size: 10485760,
}

func TestReadTruncatedOTAImageA(t *testing.T) {
	testF := fmt.Sprintf("testdata/%s", truncatedArtifact.Name)
	b, err := openTestFile(testF)
	if err != nil {
		t.Fatalf("failed to open test files")
	}
	defer b.Close()

	err = processTestFile(b)
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
	testF := fmt.Sprintf("testdata/%s", damangedOTAImageArtifact.Name)
	b, err := openTestFile(testF)
	if err != nil {
		t.Error(err)
	}
	defer b.Close()

	err = processTestFile(b)
	// Expect to see unexpected EOF
	if err != ErrChecksum {
		t.Errorf("expected to get %v, but get %v", io.ErrUnexpectedEOF, err)
	}
}
