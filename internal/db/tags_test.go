package db

import (
	"reflect"
	"testing"
)

// Tags come most used first, ties alphabetical; unpublished entries count
// only when they are visible.
func TestTagsByUse(t *testing.T) {
	entries := []Journal{
		{Tag1: "Log", Tag2: "Dev", Published: true},
		{Tag1: "Dev", Tag2: "Art", Published: true},
		{Tag1: "Dev", Tag2: "Log", Tag3: "Zen", Published: true},
		{Tag1: "Bee", Published: true},
		{Tag1: "Secret", Tag2: "Bee", Tag3: "Bee2", Published: false},
	}
	if got, want := tagsByUse(entries, false), []string{"Dev", "Log", "Art", "Bee", "Zen"}; !reflect.DeepEqual(got, want) {
		t.Errorf("public: got %v, want %v", got, want)
	}
	if got, want := tagsByUse(entries, true), []string{"Dev", "Bee", "Log", "Art", "Bee2", "Secret", "Zen"}; !reflect.DeepEqual(got, want) {
		t.Errorf("author: got %v, want %v", got, want)
	}
}
