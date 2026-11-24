// Command icongen scales the site's favicon (assets/favicon.png, the "RI"
// monogram) up into the app icons for the web app manifest and iOS:
//
//   - icon-192 / icon-512 ("any"): the whole favicon, frame included.
//   - icon-maskable-512 and apple-touch-icon: without the frame, on the
//     favicon's cream. Android and iOS cut these to their own shapes, which
//     would clip a frame unevenly; the monogram is kept inside the safe zone
//     (the central circle of 80% diameter).
//
// Scaling is Catmull-Rom bicubic: smooth, slightly soft at 512 px from a 96 px
// source. Pure Go (CGO_ENABLED=0) and deterministic: rerun with `mage icons`
// after changing the favicon, and commit the PNGs it writes to assets/icons/.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const (
	source = "assets/favicon.png"
	// frameInset crops the favicon's dark frame, rounded corners included, in
	// source pixels. The frame is ~2 px on the edges but its rounded corners
	// reach further in; "RI" itself spans about x 19-76, y 20-75 of 96, so 12
	// clears the corners with a wide margin from the letters.
	frameInset = 12
	// monogramFill is how much of a frameless icon the cropped favicon spans;
	// the rest is cream. "RI" then spans ~0.55 of the icon (box diagonal ~0.78),
	// inside the maskable safe circle of 0.8.
	monogramFill = 0.70
)

type icon struct {
	name      string
	size      int
	frameless bool
}

var icons = []icon{
	{"icon-192.png", 192, false},
	{"icon-512.png", 512, false},
	{"icon-maskable-512.png", 512, true},
	{"apple-touch-icon.png", 180, true},
}

func main() {
	out := "assets/icons"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	src, err := load(source)
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fail(err)
	}
	for _, ic := range icons {
		var img *image.NRGBA
		if ic.frameless {
			img = frameless(src, ic.size)
		} else {
			img = scale(src, src.Bounds(), ic.size, ic.size)
		}
		path := filepath.Join(out, ic.name)
		if err := write(path, img); err != nil {
			fail(err)
		}
		fmt.Println("wrote", path)
	}
}

// frameless puts the favicon's inner area (frame cropped off) at the centre of
// a cream square of the given size.
func frameless(src image.Image, size int) *image.NRGBA {
	b := src.Bounds()
	inner := image.Rect(b.Min.X+frameInset, b.Min.Y+frameInset, b.Max.X-frameInset, b.Max.Y-frameInset)
	cream := color.NRGBAModel.Convert(src.At(inner.Min.X+2, inner.Min.Y+2)).(color.NRGBA)

	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for i := 0; i < len(dst.Pix); i += 4 {
		dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = cream.R, cream.G, cream.B, 0xff
	}
	side := int(math.Round(float64(size) * monogramFill))
	off := (size - side) / 2
	part := scale(src, inner, side, side)
	for y := 0; y < side; y++ {
		copy(dst.Pix[(off+y)*dst.Stride+off*4:(off+y)*dst.Stride+(off+side)*4], part.Pix[y*part.Stride:y*part.Stride+side*4])
	}
	return dst
}

// scale resamples the rectangle r of src to w x h with Catmull-Rom bicubic
// interpolation (edges clamped). The source is opaque, so no premultiplying.
func scale(src image.Image, r image.Rectangle, w, h int) *image.NRGBA {
	sw, sh := r.Dx(), r.Dy()
	px := func(x, y int) [3]float64 {
		x = clamp(x, 0, sw-1) + r.Min.X
		y = clamp(y, 0, sh-1) + r.Min.Y
		c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
		return [3]float64{float64(c.R), float64(c.G), float64(c.B)}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		fy := (float64(y)+0.5)*float64(sh)/float64(h) - 0.5
		y0 := int(math.Floor(fy))
		ty := fy - float64(y0)
		for x := 0; x < w; x++ {
			fx := (float64(x)+0.5)*float64(sw)/float64(w) - 0.5
			x0 := int(math.Floor(fx))
			tx := fx - float64(x0)
			var acc [3]float64
			for j := -1; j <= 2; j++ {
				wy := catmullRom(float64(j) - ty)
				for i := -1; i <= 2; i++ {
					wgt := wy * catmullRom(float64(i)-tx)
					p := px(x0+i, y0+j)
					for k := 0; k < 3; k++ {
						acc[k] += wgt * p[k]
					}
				}
			}
			o := dst.PixOffset(x, y)
			for k := 0; k < 3; k++ {
				dst.Pix[o+k] = uint8(math.Round(math.Max(0, math.Min(255, acc[k]))))
			}
			dst.Pix[o+3] = 0xff
		}
	}
	return dst
}

// catmullRom is the Catmull-Rom cubic kernel (a = -0.5).
func catmullRom(t float64) float64 {
	t = math.Abs(t)
	switch {
	case t < 1:
		return 1.5*t*t*t - 2.5*t*t + 1
	case t < 2:
		return -0.5*t*t*t + 2.5*t*t - 4*t + 2
	default:
		return 0
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func write(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "icongen:", err)
	os.Exit(1)
}
