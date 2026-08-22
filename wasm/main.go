//go:build js && wasm

// Command qrwasm exposes the decoder to a browser so a page can read QR codes
// with nothing installed.
//
// It deliberately does not decode image *files*. The browser already has
// decoders for everything it can display — JPEG, PNG, WebP, AVIF, GIF, and on
// Safari/macOS even HEIC — and they are native code. Go's own decoders would
// only add weight, and gen2brain/heic under GOOS=js takes minutes per photo.
// So JavaScript hands over raw RGBA pixels and this side does the one thing
// the browser cannot: find and read the codes.
package main

import (
	"image"
	"syscall/js"

	"github.com/thousandflowers/qr-multi-imgs/scanner"
)

// decode reads every QR code out of one frame of RGBA pixels.
//
// JS signature: qrDecode(width, height, Uint8ClampedArray) -> {codes, error}
// It is synchronous and CPU-bound, which is why the page runs it inside a
// worker rather than on the thread that has to keep the UI moving.
func decode(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return result(nil, "decode(width, height, pixels) takes three arguments")
	}
	w, h := args[0].Int(), args[1].Int()
	if w <= 0 || h <= 0 {
		return result(nil, "image has no pixels")
	}

	want := w * h * 4
	if n := args[2].Length(); n != want {
		return result(nil, "pixel buffer is the wrong size for the given dimensions")
	}

	pix := make([]byte, want)
	js.CopyBytesToGo(pix, args[2])
	img := &image.RGBA{Pix: pix, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}

	codes, err := scanner.ScanDecodedImage(img)
	if err != nil {
		return result(nil, err.Error())
	}
	return result(codes, "")
}

func result(codes []string, errMsg string) any {
	out := make([]any, len(codes))
	for i, c := range codes {
		out[i] = c
	}
	m := map[string]any{"codes": out}
	if errMsg != "" {
		m["error"] = errMsg
	}
	return m
}

func main() {
	js.Global().Set("qrDecode", js.FuncOf(decode))
	// Tell the loader the module is live; without this the page cannot know
	// whether it is safe to send work.
	if ready := js.Global().Get("qrWasmReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {} // a Go wasm main that returns takes its exported functions with it
}
