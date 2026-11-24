package paging

import (
	"reflect"
	"testing"
)

func TestSlice(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}
	cases := []struct {
		name      string
		requested int
		want      []int
		page      Page
	}{
		{"first", 1, []int{1, 2, 3}, Page{Number: 1, Count: 3, Size: 3, Total: 7}},
		{"middle", 2, []int{4, 5, 6}, Page{Number: 2, Count: 3, Size: 3, Total: 7}},
		{"short last", 3, []int{7}, Page{Number: 3, Count: 3, Size: 3, Total: 7}},
		{"past the end clamps to last", 99, []int{7}, Page{Number: 3, Count: 3, Size: 3, Total: 7}},
		{"zero clamps to first", 0, []int{1, 2, 3}, Page{Number: 1, Count: 3, Size: 3, Total: 7}},
		{"negative clamps to first", -4, []int{1, 2, 3}, Page{Number: 1, Count: 3, Size: 3, Total: 7}},
	}
	for _, c := range cases {
		got, page := Slice(items, c.requested, 3)
		if !reflect.DeepEqual(got, c.want) || page != c.page {
			t.Errorf("%s: Slice(…, %d, 3) = %v %+v, want %v %+v", c.name, c.requested, got, page, c.want, c.page)
		}
	}

	got, page := Slice([]int(nil), 5, 3)
	if len(got) != 0 || page != (Page{Number: 1, Count: 1, Size: 3, Total: 0}) {
		t.Errorf("empty list: got %v %+v, want one empty page", got, page)
	}
	if page.IsPaged() || page.HasPrev() || page.HasNext() {
		t.Errorf("an empty list has no other pages")
	}

	_, exact := Slice(items[:6], 2, 3)
	if exact.Count != 2 || exact.HasNext() || !exact.HasPrev() {
		t.Errorf("exact multiple: %+v", exact)
	}
}

func TestParse(t *testing.T) {
	for raw, want := range map[string]int{"": 1, "abc": 1, "0": 1, "-2": 1, "1": 1, "7": 7, "2.5": 1, "999999": 999999} {
		if got := Parse(raw); got != want {
			t.Errorf("Parse(%q) = %d, want %d", raw, got, want)
		}
	}
}
