# ota-image-libs-go

Go libraries for streaming OTA Image version 1 artifact.

This library implements std tar library like interface for streaming files from an sequential IO stream that serving OTA image artifact.

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
	for i := 0; ; {
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