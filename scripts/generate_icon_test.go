package main

import (
	"image/color"
	"testing"
)

func TestMakeIconReturnsMasked1024Canvas(t *testing.T) {
	img := makeIcon(1024)

	if got := img.Bounds().Dx(); got != 1024 {
		t.Fatalf("width = %d, want 1024", got)
	}
	if got := img.Bounds().Dy(); got != 1024 {
		t.Fatalf("height = %d, want 1024", got)
	}

	if alphaAt(img.At(0, 0)) != 0 {
		t.Fatalf("top-left corner should be transparent")
	}
	if alphaAt(img.At(512, 512)) < 240 {
		t.Fatalf("center hub should be opaque")
	}
	if alphaAt(img.At(512, 80)) < 240 {
		t.Fatalf("top edge inside rounded square should be opaque")
	}
}

func alphaAt(c color.Color) uint32 {
	_, _, _, a := c.RGBA()
	return a >> 8
}
