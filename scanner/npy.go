package scanner

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"strings"
)

// npyHeader describes the header of a .npy v1.0 file.
type npyHeader struct {
	Descr        string `json:"descr"`
	FortranOrder bool   `json:"fortran_order"`
	Shape        []int  `json:"shape"`
}

// readBoolNPY reads a 2D boolean array from a .npy v1.0 file.
func readBoolNPY(path string) ([][]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 10 {
		return nil, fmt.Errorf("truncated npy file")
	}
	if string(data[0:6]) != "\x93NUMPY" {
		return nil, fmt.Errorf("bad magic: %x", data[:6])
	}
	if data[6] != 1 {
		return nil, fmt.Errorf("unsupported npy version: %d", data[6])
	}

	hdrLen := int(binary.LittleEndian.Uint16(data[8:10]))
	if 10+hdrLen > len(data) {
		return nil, fmt.Errorf("header truncated")
	}

	var hdr npyHeader
	hdrStr := pyDictToJSON(string(data[10 : 10+hdrLen]))
	if err := json.Unmarshal([]byte(hdrStr), &hdr); err != nil {
		return nil, fmt.Errorf("header %q: %w", hdrStr, err)
	}
	if len(hdr.Shape) != 2 {
		return nil, fmt.Errorf("expected 2D array, got %dD", len(hdr.Shape))
	}

	rows, cols := hdr.Shape[0], hdr.Shape[1]
	buf := data[10+hdrLen:]

	result := make([][]bool, rows)
	for i := range result {
		result[i] = make([]bool, cols)
		for j := 0; j < cols; j++ {
			var idx int
			if hdr.FortranOrder {
				idx = j*rows + i
			} else {
				idx = i*cols + j
			}
			result[i][j] = buf[idx] != 0
		}
	}
	return result, nil
}

// pyDictToJSON converts a Python dict literal (npy header format) to JSON.
// NumPy uses Python repr: {'descr': '|b1', 'fortran_order': False, 'shape': (109, 109), }
func pyDictToJSON(s string) string {
	s = strings.TrimRight(s, " \n\r\t")
	s = strings.ReplaceAll(s, "True", "true")
	s = strings.ReplaceAll(s, "False", "false")
	s = strings.ReplaceAll(s, "(", "[")
	s = strings.ReplaceAll(s, ")", "]")
	var b strings.Builder
	inQuote := false
	for _, r := range s {
		if r == '\'' {
			b.WriteRune('"')
		} else {
			if r == '"' {
				inQuote = !inQuote
			}
			b.WriteRune(r)
		}
	}
	s = b.String()
	if idx := strings.LastIndex(s, ","); idx >= 0 {
		after := strings.TrimSpace(s[idx+1:])
		if after == "}" || after == "]" {
			s = s[:idx] + s[idx+1:]
		}
	}
	return s
}

// renderMask renders a boolean 2D mask as a clean high-resolution grayscale image.
// Adds a quiet zone of 4 modules on each side so the decoder can find patterns.
func renderMask(mask [][]bool, scale int) *image.Gray {
	h, w := len(mask), len(mask[0])
	pad := 4 * scale
	dw, dh := w*scale+2*pad, h*scale+2*pad
	dst := image.NewGray(image.Rect(0, 0, dw, dh))
	// All white (quiet zone)
	for i := range dst.Pix {
		dst.Pix[i] = 255
	}
	// Render black modules
	for my := 0; my < h; my++ {
		for mx := 0; mx < w; mx++ {
			if !mask[my][mx] {
				continue
			}
			baseY := my*scale + pad
			baseX := mx*scale + pad
			for dy := 0; dy < scale; dy++ {
				row := dst.Pix[(baseY+dy)*dst.Stride+baseX : (baseY+dy)*dst.Stride+baseX+scale]
				for dx := range row {
					row[dx] = 0
				}
			}
		}
	}
	return dst
}
