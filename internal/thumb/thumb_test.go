package thumb

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"testing"
)

func TestSourceKnowsWhichLinksHaveAStill(t *testing.T) {
	for _, c := range []struct{ url, kind, src string }{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "video", "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg"},
		{"https://youtu.be/dQw4w9WgXcQ", "video", "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg"},
		{"https://youtube.com/shorts/dQw4w9WgXcQ", "video", "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg"},
		{"https://vimeo.com/76979871", "video", "vimeo:76979871"},
		{"https://example.org/funny.GIF", "gif", "https://example.org/funny.GIF"},
		{"https://example.org/a/photo.webp?x=1", "image", "https://example.org/a/photo.webp?x=1"},
		{"https://example.org/clip.mp4", "", ""},
		{"https://x.com/a/status/1", "", ""},
		{"https://example.org/page", "", ""},
		{"ftp://example.org/a.png", "", ""},
		{"https://www.youtube.com/watch?v=<x>", "", ""},
	} {
		if kind, src := Source(c.url); kind != c.kind || src != c.src {
			t.Errorf("Source(%q) = %q %q, want %q %q", c.url, kind, src, c.kind, c.src)
		}
	}
}

func TestPublicOnlyRefusesInternalAddresses(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:80", "[::1]:443", "10.0.0.5:80", "192.168.1.1:80", "172.16.0.1:80",
		"169.254.169.254:80", "100.64.0.1:80", "0.0.0.0:80", "[fe80::1]:80", "[fd00::1]:80", "nonsense"} {
		if err := publicOnly("tcp", addr, nil); err != ErrBlocked {
			t.Errorf("publicOnly(%q) = %v, want refused", addr, err)
		}
	}
	for _, addr := range []string{"142.250.80.46:443", "[2606:4700::1111]:443"} {
		if err := publicOnly("tcp", addr, nil); err != nil {
			t.Errorf("publicOnly(%q) = %v, want allowed", addr, err)
		}
	}
}

func TestEncodeShrinksToCardWidthAndKeepsTheFirstFrame(t *testing.T) {
	frame := func(c color.Color) *image.Paletted {
		p := image.NewPaletted(image.Rect(0, 0, 1360, 1020), color.Palette{color.Black, color.White, color.RGBA{255, 0, 0, 255}})
		for y := 0; y < 1020; y++ {
			for x := 0; x < 1360; x++ {
				p.Set(x, y, c)
			}
		}
		return p
	}
	var g bytes.Buffer
	if err := gif.EncodeAll(&g, &gif.GIF{Image: []*image.Paletted{frame(color.RGBA{255, 0, 0, 255}), frame(color.White)}, Delay: []int{10, 10}}); err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(&g)
	if err != nil {
		t.Fatal(err)
	}
	out, w, h, err := encode(img)
	if err != nil || w != Width || h != 510 {
		t.Fatalf("encode: %dx%d %v", w, h, err)
	}
	back, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if r, gr, b, _ := back.At(w/2, h/2).RGBA(); r>>8 < 200 || gr>>8 > 60 || b>>8 > 60 {
		t.Errorf("the still must be the first (red) frame, got %d %d %d", r>>8, gr>>8, b>>8)
	}
	small := image.NewRGBA(image.Rect(0, 0, 200, 100))
	if _, w, h, _ := encode(small); w != 200 || h != 100 {
		t.Errorf("a small image is never enlarged: got %dx%d", w, h)
	}
}
