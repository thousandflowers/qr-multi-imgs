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

	"github.com/makiuchi-d/gozxing/qrcode/decoder"
	"github.com/makiuchi-d/gozxing/qrcode/encoder"
	"github.com/thousandflowers/qr-multi-imgs/scanner"
)

// decode reads every QR code out of one frame of RGBA pixels.
//
// JS signature:
//
//	qrDecode(width, height, Uint8ClampedArray)
//	  -> {codes, classification, detections, metadata, error}
//
// It is synchronous and CPU-bound, which is why the page runs it inside a
// worker rather than on the thread that has to keep the UI moving.
//
// classification is why the answer is what it is — see scanner.Classification.
// detections carry a box each, and only when a code was located and could not
// be read. The page does not draw them; they are here because the same shape
// is what the CLI exports and what the local API returns, and a caller that
// wants them should not have to decode the image a second time to get them.
//
// Boxes are in the pixel space of the frame handed in, which is not the
// original file's when the caller shrank it first. Any consumer that wants
// them in file coordinates has to scale them itself, and only the caller still
// knows by how much.
//
// metadata carries the same caveat and one more: its dimensions and image
// measures describe the frame as handed over, so a page that decodes at a cap
// is measuring the shrunk image. Laplacian variance in particular is
// scale-dependent and comparing it across differently-capped runs is
// meaningless. The dimensions travel with the numbers so that is checkable.
func decode(_ js.Value, args []js.Value) any {
	img, argErr := frameFrom(args)
	if argErr != "" {
		return errResult(argErr)
	}
	d, err := scanner.ScanDecodedImageDetail(img)
	if err != nil {
		return errResult(err.Error())
	}
	return result(d)
}

func result(d scanner.Detail) any {
	codes := make([]any, len(d.Codes))
	for i, c := range d.Codes {
		codes[i] = c
	}
	// The box is nested, exactly as scanner.Detection marshals it for the JSON
	// export and the local API. This used to flatten x/y/w/h into the detection
	// while the docstring above claimed the shape matched the other surfaces;
	// it did not, and a page consuming both would have had to know which one it
	// was talking to.
	dets := make([]any, len(d.Detections))
	for i, det := range d.Detections {
		dets[i] = map[string]any{
			"box": map[string]any{
				"x": det.Box.X, "y": det.Box.Y, "w": det.Box.W, "h": det.Box.H,
			},
			"module_size": det.ModuleSize,
			"version":     det.Version,
			// Empty when the code was located and not read. A viewfinder draws
			// those in a different colour from the ones it could read.
			"text": det.Text,
		}
	}
	out := map[string]any{
		"codes":          codes,
		"classification": string(d.Classification),
		"detections":     dets,
	}
	if m := d.Metadata; m != nil {
		strategies := make([]any, len(m.Strategies))
		for i, s := range m.Strategies {
			strategies[i] = map[string]any{"name": s.Name, "found": s.Found}
		}
		out["metadata"] = map[string]any{
			"width":              m.Width,
			"height":             m.Height,
			"laplacian_variance": m.LaplacianVariance,
			"edge_density":       m.EdgeDensity,
			"strategies":         strategies,
		}
	}
	return out
}

func errResult(msg string) any {
	return map[string]any{
		"codes":      []any{},
		"detections": []any{},
		"error":      msg,
	}
}

// encode turns a payload back into a QR code, as the module matrix rather than
// as pixels.
//
// JS signature:
//
//	qrEncode(text) -> {size, bits}   // bits is size*size of 0/1, row-major
//
// A matrix rather than an image because the page wants four different outputs
// from it - PNG, JPEG, SVG and PDF - and three of those are better built from
// modules than from a raster someone has to trace back. The browser draws it;
// this side only says which squares are black.
//
// The encoder is gozxing's, already a dependency for reading. Nothing new was
// added to draw with.
func encode(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return map[string]any{"error": "encode(text) takes one argument"}
	}
	text := args[0].String()
	if text == "" {
		return map[string]any{"error": "nothing to encode"}
	}
	// Error correction M: the middle of the four levels, and the one a
	// recreated code wants - high enough to survive a print and a phone camera,
	// low enough not to inflate a long payload into a denser code than the
	// original.
	qr, err := encoder.Encoder_encode(text, decoder.ErrorCorrectionLevel_M, nil)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	m := qr.GetMatrix()
	w, h := m.GetWidth(), m.GetHeight()
	if w != h {
		return map[string]any{"error": "encoder returned a non-square matrix"}
	}
	bits := make([]any, 0, w*h)
	for y := range h {
		for x := range w {
			if m.Get(x, y) == 1 {
				bits = append(bits, 1)
			} else {
				bits = append(bits, 0)
			}
		}
	}
	return map[string]any{"size": w, "bits": bits}
}

// decodeLive is decode for a viewfinder: the short cascade, no image measures.
//
// Same arguments, same result shape. What differs is what it is willing to
// spend - see scanner/live.go for the measurements that made a separate path
// necessary rather than a nice idea.
func decodeLive(_ js.Value, args []js.Value) any {
	img, err := frameFrom(args)
	if err != "" {
		return errResult(err)
	}
	return result(scanner.ScanLive(img))
}

// frameFrom is the argument checking both entry points share.
func frameFrom(args []js.Value) (*image.RGBA, string) {
	if len(args) < 3 {
		return nil, "decode(width, height, pixels) takes three arguments"
	}
	w, h := args[0].Int(), args[1].Int()
	if w <= 0 || h <= 0 {
		return nil, "image has no pixels"
	}
	want := w * h * 4
	if n := args[2].Length(); n != want {
		return nil, "pixel buffer is the wrong size for the given dimensions"
	}
	pix := make([]byte, want)
	js.CopyBytesToGo(pix, args[2])
	return &image.RGBA{Pix: pix, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}, ""
}

func main() {
	js.Global().Set("qrDecode", js.FuncOf(decode))
	js.Global().Set("qrDecodeLive", js.FuncOf(decodeLive))
	js.Global().Set("qrEncode", js.FuncOf(encode))
	// Tell the loader the module is live; without this the page cannot know
	// whether it is safe to send work.
	if ready := js.Global().Get("qrWasmReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {} // a Go wasm main that returns takes its exported functions with it
}
