// Package paging splits an ordered list into fixed-size pages.
//
// Requests are untrusted (?page=abc, ?page=-3, ?page=999), so a Page is only
// ever made by Slice, which clamps the request into range: a Page's Number is
// always a real page, 1 <= Number <= Count.
package paging

import "strconv"

// Page describes one page of a list. The zero value is not meaningful; use Slice.
type Page struct {
	Number int // 1-based, always within 1..Count
	Count  int // number of pages, at least 1 (an empty list has one, empty, page)
	Size   int // items per page
	Total  int // items across all pages
}

// HasPrev reports whether a page comes before this one.
func (p Page) HasPrev() bool { return p.Number > 1 }

// HasNext reports whether a page comes after this one.
func (p Page) HasNext() bool { return p.Number < p.Count }

// IsPaged reports whether the list spans more than one page.
func (p Page) IsPaged() bool { return p.Count > 1 }

// Parse reads a requested page number from a query value. Anything that is not
// a positive integer means page 1; Slice clamps values past the end.
func Parse(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// Slice returns the items on the requested page and the page's description.
// A request past the last page gets the last page (e.g. after deletions).
// size must be positive.
func Slice[T any](items []T, requested, size int) ([]T, Page) {
	if size < 1 {
		panic("paging: size must be positive")
	}
	total := len(items)
	count := (total + size - 1) / size
	if count < 1 {
		count = 1
	}
	n := requested
	if n < 1 {
		n = 1
	}
	if n > count {
		n = count
	}
	start := (n - 1) * size
	end := start + size
	if end > total {
		end = total
	}
	return items[start:end], Page{Number: n, Count: count, Size: size, Total: total}
}
