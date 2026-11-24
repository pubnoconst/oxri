package linen

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// The two tiles must be coprime, or the pair repeats at their LCM, far sooner.
func TestTilesAreCoprime(t *testing.T) {
	if gcd(Near, Far) != 1 {
		t.Fatalf("Near %d and Far %d share a factor; the weave would repeat early", Near, Far)
	}
}

// Every cloth is defined once, as a Near tile then a Far tile, each a
// stitched (seamless) noise tile of its declared size, with distinct seeds.
func TestEveryClothHasTwoLayers(t *testing.T) {
	size := regexp.MustCompile(`width='(\d+)' height='(\d+)'%3E%3Cfilter`)
	seed := regexp.MustCompile(`seed='(\d+)'`)
	for _, name := range []string{"--linen-page-light", "--linen-card-light", "--linen-page-dark", "--linen-card-dark"} {
		if n := strings.Count(CSS, "  "+name+": "); n != 1 {
			t.Fatalf("%s defined %d times", name, n)
		}
		line := CSS[strings.Index(CSS, "  "+name+": "):]
		line = line[:strings.Index(line, ";\n")]
		sizes := size.FindAllStringSubmatch(line, -1)
		if len(sizes) != 2 || sizes[0][1] != fmt.Sprint(Near) || sizes[1][1] != fmt.Sprint(Far) {
			t.Errorf("%s: want a %d tile then a %d tile, got %v", name, Near, Far, sizes)
		}
		if strings.Count(line, "stitchTiles='stitch'") != 4 {
			t.Errorf("%s: every noise must be stitched, or tile edges show", name)
		}
		seen := map[string]bool{}
		for _, s := range seed.FindAllStringSubmatch(line, -1) {
			if seen[s[1]] {
				t.Errorf("%s: seed %s used twice; the layers would share threads", name, s[1])
			}
			seen[s[1]] = true
		}
	}
}

// Offsets are stable per key, stay within each tile, and differ between cards.
func TestOffsetIsSeededPerCard(t *testing.T) {
	render := func(key string) string {
		var b strings.Builder
		_ = Offset(key).Render(&b)
		return b.String()
	}
	if render("entry:1") != render("entry:1") {
		t.Fatal("the same card must always get the same cloth")
	}
	pos := regexp.MustCompile(`background-position: -(\d+)px -(\d+)px, -(\d+)px -(\d+)px, 0 0`)
	distinct := map[string]bool{}
	for i := 0; i < 200; i++ {
		out := render(fmt.Sprintf("entry:%d", i))
		m := pos.FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("unexpected offset %q", out)
		}
		var v [4]int
		for j := range v {
			fmt.Sscan(m[j+1], &v[j])
		}
		if v[0] >= Near || v[1] >= Near || v[2] >= Far || v[3] >= Far {
			t.Errorf("offset outside its tile: %v", v)
		}
		distinct[out] = true
	}
	if len(distinct) < 195 {
		t.Errorf("only %d distinct offsets over 200 cards", len(distinct))
	}
}
