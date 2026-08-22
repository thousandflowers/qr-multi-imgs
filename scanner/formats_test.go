package scanner

import (
	"strings"
	"testing"
)

// The portable set is the floor everywhere, whatever the system adds on top.
func TestSupportsExtension_portableSet(t *testing.T) {
	for _, name := range []string{"a.png", "a.PNG", "b.jpg", "c.jpeg", "d.gif", "e.bmp", "f.webp", "g.tiff", "h.tif"} {
		if !SupportsExtension(name) {
			t.Errorf("%s should be scanned everywhere", name)
		}
	}
	for _, name := range []string{"notes.txt", "mask.npy", "archive.zip", "noextension"} {
		if SupportsExtension(name) {
			t.Errorf("%s should not be picked up by a folder walk", name)
		}
	}
}

// On a Mac the readable formats come from ImageIO rather than a list in this
// repo, which is what makes camera raw work without naming every camera.
func TestSystemExtensions_areMergedIn(t *testing.T) {
	sys := systemExtensions()
	if len(sys) == 0 {
		t.Skip("no system decoder on this platform or build (CGO_ENABLED=0)")
	}
	for _, ext := range sys {
		if !SupportsExtension("file." + ext) {
			t.Errorf("system reports it can read .%s, but a folder scan would skip it", ext)
		}
	}

	// Assert on the capability rather than on a named format: what the merge
	// buys is the formats Go's image package cannot open, and if none is
	// reported it is buying nothing.
	beyondGo := 0
	for _, ext := range sys {
		switch ext {
		case "png", "jpeg", "jpg", "gif", "bmp", "tiff", "tif", "webp":
		default:
			beyondGo++
		}
	}
	if beyondGo == 0 {
		t.Error("the system reported nothing beyond the formats Go already decodes")
	}
	t.Logf("%d system formats merged in: %s", len(sys), strings.Join(sys, " "))
}
