# ota-image-libs-go

Go libraries for streaming OTA Image version 1 artifact.

This library implements std tar library like interface for streaming files from an sequential IO stream that serving OTA image artifact.
Note that this library only guarantee properly streaming OTA image artifact!

## OTA Image version 1 artifact

Image artifact of OTA Image version 1 is a strict subset of ZIP archive, which has the following constrains:

1. all file entries(blobs) don't have compression via ZIP, stored as plain file(compression is done during OTA image build, not by artifact packing).

2. all file entries have fixed permission bit and datetime set.

3. all file entries have size less than 32MiB (with exceptions when otaclient client update backward compatibility is enabled, but the extra files(otaclient release package) will still be much smaller than 4GiB).

For details, see https://github.com/tier4/ota-image-builder/blob/main/src/ota_image_builder/cmds/pack_artifact.py.

## Usage

This library exposes `Reader` for handling the OTA image artifact IO stream,
the instance of `Reader` implements `Next` and `Read` API, similar to std `tar` package.

Caller needs to first call `Next` to get the next file's local file header,
and then `Read` API is ready for caller to read util the end of the corresponding file entry.

Repeat the `Next` and `Read` calling until `Next` returns EOF, we can stream throught the whole artifact.

## Get started

Installation:

```shell
go get github.com/tier4/ota-image-libs-go/artifact
```

Example:

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/tier4/ota-image-libs-go/artifact"
)

const fileToRead = "ota_image.zip"

func main() {
	fmt.Printf("will read %s\n", fileToRead)

	f, err := os.Open(fileToRead)
	if err != nil {
		fmt.Printf("failed to open %s: %s", fileToRead, err)
	}
	defer f.Close()

	r := artifact.NewReader(f)

	buf := make([]byte, 1024*1024) // 1MiB
	var i int
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			fmt.Printf("finish up reading! Total %d files read", i)
			return
		}
		if !hdr.IsDir() {
			i += 1
		}

		if err != nil {
			fmt.Printf("failed during streaming: %s", err)
			return
		}
		fmt.Printf("This header #%d: %v\n", i, hdr)

        // reading the data of the file
		for {
			_, err := r.Read(buf)
			if err == io.EOF {
				break
			}
		}
	}
}
```
