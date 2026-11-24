// Package linen draws the site's woven-cloth surfaces and serves them as a
// small stylesheet of CSS custom properties (--linen-page-light, …), which
// site.css switches between for light and dark.
//
// Each cloth is two layers of SVG noise on tiles of coprime sizes (Near and
// Far, both prime). One tile repeats every few hundred pixels and the eye
// finds the grid; two, offset against each other, only line up again every
// Near×Far pixels, so the weave never visibly repeats. Each layer carries part
// of the contrast, so the cloth reads as strong as a single layer did.
//
// Journal cards each start the weave at their own seeded point (Offset), so
// no two cards show the same piece of cloth.
//
// Cost: the browser rasterises each tile's noise once per surface. Near²+Far²
// is about 3.4× the old single 280 px tile, far less than one big tile with
// the same freedom from repetition.
package linen

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/oxri/oxri-go/assets"
	g "maragu.dev/gomponents"
)

// Tile sizes in CSS pixels. Coprime (both prime), so the pair repeats only
// every Near*Far pixels.
const (
	Near = 307
	Far  = 419
)

// layerStrength scales each layer's contrast: two independent layers at
// 0.75 sum to about the contrast of one layer at full strength (0.75·√2).
const layerStrength = 0.75

// weave describes one cloth: the noise grain (horizontal and vertical thread
// frequencies), the seeds of its threads, and how dark and light its threads
// go (alpha amplitude of the dark and light passes).
type weave struct {
	name         string
	along, cross float64 // baseFrequency along a thread and across it
	seeds        [2]int  // horizontal and vertical threads
	dark, light  float64
}

// The four cloths. Grain and contrast are the values the single-tile linen
// used (RATIONALE history); seeds differ per cloth so page and card never
// share a weave.
var weaves = []weave{
	{"--linen-page-light", 0.9, 0.055, [2]int{5, 11}, 0.10, 0.22},
	{"--linen-card-light", 0.9, 0.055, [2]int{3, 17}, 0.10, 0.22},
	{"--linen-page-dark", 0.9, 0.055, [2]int{7, 13}, 0.075, 0.10},
	{"--linen-card-dark", 0.6, 0.11, [2]int{3, 17}, 0.085, 0.11},
}

// tile is one layer of a cloth as an SVG data URI.
func tile(w weave, size int, seedShift int) string {
	d, l := w.dark*layerStrength, w.light*layerStrength
	svg := fmt.Sprintf(
		`<svg xmlns='http://www.w3.org/2000/svg' width='%[1]d' height='%[1]d'>`+
			`<filter id='w' x='0' y='0' width='100%%' height='100%%' color-interpolation-filters='sRGB'>`+
			`<feTurbulence type='fractalNoise' baseFrequency='%[2]g %[3]g' numOctaves='3' seed='%[4]d' stitchTiles='stitch' result='h'/>`+
			`<feTurbulence type='fractalNoise' baseFrequency='%[3]g %[2]g' numOctaves='3' seed='%[5]d' stitchTiles='stitch' result='v'/>`+
			`<feComposite in='h' in2='v' operator='arithmetic' k2='0.5' k3='0.5' result='n'/>`+
			`<feColorMatrix in='n' type='matrix' values='0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  %.4f 0 0 0 %.4f' result='d'/>`+
			`<feColorMatrix in='n' type='matrix' values='0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  %.4f 0 0 0 %.4f' result='l'/>`+
			`<feMerge><feMergeNode in='d'/><feMergeNode in='l'/></feMerge></filter>`+
			`<rect width='%[1]d' height='%[1]d' filter='url(#w)'/></svg>`,
		size, w.along, w.cross, w.seeds[0]+seedShift, w.seeds[1]+seedShift,
		d, -d/2, -l, l/2,
	)
	return `url("data:image/svg+xml,` + escaper.Replace(svg) + `")`
}

// escaper makes an SVG safe inside a CSS url("data:…") (same encoding the
// hand-written tiles used).
var escaper = strings.NewReplacer("<", "%3C", ">", "%3E", "#", "%23", "%", "%25", `"`, "%22")

// CSS is the stylesheet: each cloth is a two-layer background value, Near
// tile first, then Far.
var CSS = func() string {
	var b strings.Builder
	b.WriteString("/* Linen (views/linen): generated at start-up; do not edit by hand. */\n:root {\n")
	for _, w := range weaves {
		fmt.Fprintf(&b, "  %s: %s, %s;\n", w.name, tile(w, Near, 0), tile(w, Far, 100))
	}
	b.WriteString("}\n")
	return b.String()
}()

// StylePath is the public route of the linen stylesheet.
const StylePath = "/views/linen/linen.css"

// Style is the content-hashed handle for CSS (URL and route handler).
var Style = assets.Component(StylePath, CSS)

// Offset gives a card its own starting point in the weave, seeded by key (an
// entry's ID), as an inline background-position for its two linen layers and
// the base colour. Same key, same cloth: a card never changes on reload.
func Offset(key string) g.Node {
	h := fnv.New64a()
	_, _ = h.Write([]byte("linen:" + key))
	v := h.Sum64()
	nx, ny := v%Near, (v>>16)%Near
	fx, fy := (v>>32)%Far, (v>>48)%Far
	return g.Attr("style", fmt.Sprintf("background-position: -%dpx -%dpx, -%dpx -%dpx, 0 0", nx, ny, fx, fy))
}
