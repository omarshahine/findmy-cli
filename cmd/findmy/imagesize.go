package main

import (
	"image"
	_ "image/png"
	"io"
)

func decodeConfig(r io.Reader) (image.Config, string, error) {
	return image.DecodeConfig(r)
}
