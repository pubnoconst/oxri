package routes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/verse"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeHelloAO serves Genesis 12 in the KJV only (public domain), the way
// the provider shapes a chapter file, and counts the requests.
func fakeHelloAO(calls *atomic.Int32) *verse.Source {
	const genesis12 = `{"book":{"id":"GEN","commonName":"Genesis"},"chapter":{"number":12,"content":[
	 {"type":"verse","number":3,"content":["And I will bless them that bless thee, and curse him that curseth thee:",{"noteId":1}," and in thee shall all families of the earth be blessed."]},
	 {"type":"verse","number":4,"content":["So Abram departed, as the LORD had spoken unto him; and Lot went with him."]}]}}`
	return verse.NewSource(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Host != "bible.helloao.org" {
			return nil, fmt.Errorf("unexpected host %s", r.URL.Host)
		}
		if r.URL.Path != "/api/eng_kjv/GEN/12.json" {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(genesis12)), Header: http.Header{}}, nil
	})})
}

const gen12_3to4 = "And I will bless them that bless thee, and curse him that curseth thee: and in thee shall all families of the earth be blessed. So Abram departed, as the LORD had spoken unto him; and Lot went with him."

func scriptureForm(text string, extra url.Values) url.Values {
	f := url.Values{
		"media_kind": {"scripture"}, "scripture": {"bible"},
		"verse_book": {"GEN"}, "verse_from": {"12:3"}, "verse_to": {"4"},
		"verse_tr_choice": {"auto"}, "verse_tr": {"eng_kjv"},
		"verse_text": {text}, "body": {"Kept."}, "published": {"1"},
		"media_url": {"https://example.com/stale.png"}, // ignored for scripture
	}
	for k, v := range extra {
		f[k] = v
	}
	return f
}

func TestScriptureSavedAfterCheck(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	authSvc, session := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()
	var calls atomic.Int32
	router := NewRouter(Config{Store: store, AuthService: authSvc, Passages: fakeHelloAO(&calls)})

	post := func(path string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.AddCookie(session)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	get := func(path string) string {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(session)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Body.String()
	}

	// Refused, nothing stored.
	refused := map[string]url.Values{
		"no text":         scriptureForm("", nil),
		"forged text":     scriptureForm(gen12_3to4+" And more.", nil),
		"not in YLT":      scriptureForm(gen12_3to4, url.Values{"verse_tr": {"eng_ylt"}}),
		"no book":         scriptureForm(gen12_3to4, url.Values{"verse_book": {""}}),
		"no from":         scriptureForm(gen12_3to4, url.Values{"verse_from": {""}}),
		"no scripture":    scriptureForm(gen12_3to4, url.Values{"scripture": {""}}),
		"bad translation": scriptureForm(gen12_3to4, url.Values{"verse_tr": {"auto"}}),
	}
	for name, form := range refused {
		if rec := post("/journals", form); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: got %d, want 422", name, rec.Code)
		}
	}
	if all, _ := store.List(ctx, true); len(all) != 0 {
		t.Fatalf("refused saves stored %d entries", len(all))
	}

	// The text the preview built (white space as the browser sent it).
	if rec := post("/journals", scriptureForm("  "+strings.ReplaceAll(gen12_3to4, " so ", "  so ")+"\n", nil)); rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	entries, _ := store.List(ctx, true)
	e := entries[0]
	p, ok := e.Passage()
	if !ok || p.Reference != "Genesis 12:3-4" || p.Version != "KJV" || p.Text != gen12_3to4 || p.Translation != "eng_kjv" {
		t.Fatalf("stored passage %+v, %v", p, ok)
	}
	if e.MediaURL != "https://bible.helloao.org/api/eng_kjv/GEN/12.json" {
		t.Errorf("media link %q", e.MediaURL)
	}

	page := get(fmt.Sprintf("/journals/%d", e.ID))
	for _, want := range []string{`<p>` + gen12_3to4 + `</p>`, `passage-version">KJV`, `passage-reference">Genesis 12:3-4`} {
		if !strings.Contains(page, want) {
			t.Errorf("entry page is missing %q", want)
		}
	}

	// The edit form shows the choice again.
	form := get(fmt.Sprintf("/journals/%d/edit", e.ID))
	for _, want := range []string{`<option value="scripture" selected`, `id="verse-book-search" type="text" role="combobox"`, `value="Genesis"`, `name="verse_book" value="GEN"`,
		`name="verse_from" type="text" placeholder="12:3" maxlength="7" autocomplete="off" aria-required="true" aria-describedby="scripture-hint" value="12:3"`,
		`value="4"`, `<option value="eng_kjv" data-label="KJV" selected`} {
		if !strings.Contains(form, want) {
			t.Errorf("edit form is missing %q", want)
		}
	}

	// An edit keeping the same verses and text keeps the stored passage.
	before := calls.Load()
	if rec := post(fmt.Sprintf("/journals/%d", e.ID), scriptureForm(gen12_3to4, url.Values{"body": {"Edited."}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("update: %d", rec.Code)
	}
	if calls.Load() != before {
		t.Errorf("edit asked the provider again")
	}

	// Back to a link: the passage goes.
	if rec := post(fmt.Sprintf("/journals/%d", e.ID), url.Values{"media_kind": {"link"}, "media_url": {"https://example.com/a.png"}, "body": {"Kept."}, "verse_text": {gen12_3to4}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("switch to link: %d", rec.Code)
	}
	if got, _ := store.Get(ctx, e.ID, true); got.PassageJSON != "" || got.MediaURL != "https://example.com/a.png" {
		t.Errorf("after switching to a link: %q %q", got.MediaURL, got.PassageJSON)
	}
}

func TestNewEntryFormOffersScripture(t *testing.T) {
	authSvc, session := createTestTOTPAuth(t)
	defer func() { _ = authSvc.Close() }()
	router := NewRouter(Config{AuthService: authSvc})
	req := httptest.NewRequest(http.MethodGet, "/journals/new", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	html := rec.Body.String()
	for _, want := range []string{
		`<option value="link" selected>Link</option><option value="scripture">Scripture</option>`,
		`<option value="bible">Bible</option>`,
		`Book<span class="field-required" aria-hidden="true"> *</span>`,
		`From<span class="field-required" aria-hidden="true"> *</span>`,
		`<option value="auto" data-order="eng_ylt eng_kjv" selected>YLT, or KJV where missing</option>`,
		`<li id="book-REV" role="option" tabindex="-1" aria-selected="false" data-id="REV" data-chapters="22">Revelation</li>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("new-entry form is missing %q", want)
		}
	}
}

func TestBibleProviderAllowedByCSP(t *testing.T) {
	if !strings.Contains(ContentSecurityPolicy, "connect-src 'self' https://bible.helloao.org;") {
		t.Errorf("the editor can't fetch verses: %s", ContentSecurityPolicy)
	}
}
