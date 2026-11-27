// Package labsite serves the site from an in-memory database seeded with
// entries like real ones, for the lab (PROFILING.md). It needs no database
// credentials and never touches the real database, X or the Bible provider:
// tweets are stored snapshots without images, passages are stored text.
// Readers are signed out (the admin session only seeds).
//
// The Go lab (cmd/lab) serves it in its own process; cmd/labserver serves it
// for a lab that runs elsewhere: the Node script on a Windows host, and CI
// (Principia #9, RATIONALE 091).
package labsite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oxri/oxri-go/internal/auth"
	"github.com/oxri/oxri-go/internal/db"
	"github.com/oxri/oxri-go/internal/domain"
	"github.com/oxri/oxri-go/internal/tweet"
	"github.com/oxri/oxri-go/routes"
	"github.com/oxri/oxri-go/views/journals"
)

// Site is the seeded site: what it serves, and what it has served.
type Site struct {
	handler http.Handler
	store   *db.Store
	auth    *auth.Service
	// stop ends the server that serves the site, once cmd/labserver has set it.
	stop func()
	// imageBytes and imageCount are the photos served in place of X (a page's
	// images are cross-origin to it, so the browser reports their size as 0).
	imageBytes, imageCount atomic.Int64
}

// New seeds an in-memory store with a Book I and entries (every other one a
// tweet clipping with a photo). pageSize sets how many entries a catalog page
// holds, when above 0 (the site's own size otherwise).
func New(ctx context.Context, entries, pageSize int) (*Site, error) {
	if pageSize > 0 {
		journals.CatalogPageSize = pageSize
	}
	store, err := db.Open(ctx, ":memory:", "")
	if err != nil {
		return nil, err
	}
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		return nil, err
	}
	svc, err := auth.NewServiceWithOptions(auth.ServiceOptions{TOTPSecret: secret})
	if err != nil {
		return nil, err
	}
	session, err := svc.CreateSession()
	if err != nil {
		return nil, err
	}
	cap, ok := svc.Authenticate(session)
	if !ok {
		return nil, fmt.Errorf("labsite: no admin capability for seeding")
	}
	s := &Site{store: store, auth: svc}
	if err := seed(ctx, store, cap, entries); err != nil {
		_ = svc.Close()
		_ = store.Close()
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc(ImagePath, s.serveImage)
	mux.HandleFunc(StatsPath, s.serveStats)
	mux.HandleFunc(StopPath, s.serveStop)
	mux.Handle("/", routes.NewRouter(routes.Config{Store: store, AuthService: svc}))
	s.handler = mux
	return s, nil
}

// Handler serves the site, and at ImagePath the photos on seeded clippings.
func (s *Site) Handler() http.Handler { return s.handler }

// OnStop sets the function called when StopPath is asked. The Go lab, which
// serves the site in its own process, sets none: the site ends with it.
func (s *Site) OnStop(f func()) { s.stop = sync.OnceFunc(f) }

// Images reports how many photos were served in place of X, and their bytes in
// all.
func (s *Site) Images() (count, bytes int64) {
	return s.imageCount.Load(), s.imageBytes.Load()
}

// Close releases the site's sessions and its database.
func (s *Site) Close() {
	_ = s.auth.Close()
	_ = s.store.Close()
}

const seedProse = `Good is that which prolongs vitality; evil, that which shortens it. The difference is not moral but **causal**: an act is judged by what it sets in motion, *ex post*, and not by the intention that preceded it.

## On order

1. Men are fallen.
2. Salvation is the cure of fallenness.
3. Order is the precondition of salvation, not its substitute.

> The beginning of wisdom is the fear of consequence.

In the Greek: ἐν ἀρχῇ ἦν ὁ λόγος, καὶ ὁ λόγος ἦν πρὸς τὸν θεόν. Ἀγάπη is not sentiment but the will toward another's good.`

const seedBook = `# Book I

## On Cause

Cause and effect follow each other necessarily. The difference between the two is not ontic, but epistemic. What is a cause is an effect of the cause prior; inversely, what is an effect, is a cause of the effect *ex post*. The process of effectuation given a prior set of causes is **Causal Succession**.

## Time

Time is a construct to express an index in **Causal Succession**. Time herein is not relativistic, but a point in the chain of **Causal Succession**.

## Substance, Cosmos and Substrate

**The Substance** is that which can physically instantiate, preserve and transform state, and causally interact with other Substances through lawful alteration. It is the imagined irreducible unit of existence. Matter and Energy are different modes of Substance. Energy is Substance in motion; Matter is Substance at rest relative to an observer.

The **Cosmos** is the sum of all Substances and their states. The **Substrate** is that upon which the Cosmos is written: the lawful order that every alteration of state obeys.

## On Knowing

To know is to hold a model of **Causal Succession** that predicts what follows from what. Knowledge is therefore never of things in themselves, but of their order. The Greeks called the order λόγος; the word survives in every science that ends in *-logy*.

## On Will

Will is the capacity of a Substance to select among the effects available to it. A will that selects for its own continuation is alive; a will that selects against it is in decline. Ethics is the study of such selections and their consequences across **Causal Succession**.`

// seed fills one catalog page and a book chapter: text entries quoting
// stored tweets and Bible passages, with tags, Greek and Markdown, and a
// long-form Book I.
func seed(ctx context.Context, store *db.Store, cap domain.AdminCap, entries int) error {
	if _, err := store.SaveBook(ctx, cap, "book-i", seedBook); err != nil {
		return fmt.Errorf("labsite seed: book: %w", err)
	}
	tags := [][]string{{"Good", "Truth"}, {"Caste"}, {"Law", "Order"}, {"Pray"}, {"Noise"}, {"Good"}}
	passages := []domain.Passage{
		{Translation: "eng_kjv", Book: "JHN", Chapter: 3, From: 16, To: 16, Reference: "John 3:16", Version: "KJV",
			Text: "For God so loved the world, that he gave his only begotten Son, that whosoever believeth in him should not perish, but have everlasting life."},
		{Translation: "eng_ylt", Book: "GEN", Chapter: 12, From: 3, To: 3, Reference: "Genesis 12:3", Version: "YLT",
			Text: "and I bless those blessing thee, and him who is disesteeming thee I curse, and blessed in thee have been all families of the ground."},
	}
	for i := 0; i < entries; i++ {
		body := seedProse
		if i%3 == 1 {
			body = strings.SplitN(seedProse, "\n\n## On order", 2)[0]
		}
		var entry domain.ValidatedJournalEntry
		var err error
		if i%2 == 0 {
			id := fmt.Sprintf("1000000000000%06d", i)
			entry, err = domain.ParseJournalEntry("https://x.com/res_lab/status/"+id, tags[i%len(tags)], body, i%5 == 0, true)
			if err != nil {
				return fmt.Errorf("labsite seed: entry %d: %w", i, err)
			}
			snap, err := tweet.Snapshot{
				ID: id, Name: "Res Lab", Handle: "res_lab", Created: time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC),
				Text: "What is good? A question older than the Republic, and no closer to an answer for being asked online.",
				// A photo on every clipping, as on most real ones; the site
				// serves it (ImagePath) in place of X.
				Media: []tweet.Media{{Kind: tweet.Photo, URL: fmt.Sprintf("https://pbs.twimg.com/media/lab%06d.jpg", i), Width: 1200, Height: 900}},
			}.Encode()
			if err != nil {
				return err
			}
			if err := store.PutTweetSnapshot(ctx, id, db.TweetOK, snap); err != nil {
				return fmt.Errorf("labsite seed: snapshot %d: %w", i, err)
			}
		} else {
			p := passages[(i/2)%len(passages)]
			entry, err = domain.ParseJournalEntry("https://bible.helloao.org/api/"+p.Translation+"/"+p.Book+"/"+fmt.Sprint(p.Chapter)+".json", tags[i%len(tags)], body, false, true)
			if err == nil {
				entry, err = entry.WithPassage(p)
			}
			if err != nil {
				return fmt.Errorf("labsite seed: entry %d: %w", i, err)
			}
		}
		if _, err := store.CreateValidated(ctx, cap, entry); err != nil {
			return fmt.Errorf("labsite seed: create %d: %w", i, err)
		}
	}
	return nil
}

// ImagePath serves the photos on seeded clippings. The lab sends the page's
// requests for X's images (pbs.twimg.com) here instead, through the emulated
// phone network, so image bytes cost what they would. "?name=small" gets X's
// small size (680 px wide), anything else the large one (1200 px), as X serves
// them.
const ImagePath = "/__lab/image"

// StatsPath reports, as JSON, how many photos were served at ImagePath and
// their bytes in all: {"count": n, "bytes": n}.
const StatsPath = "/__lab/stats"

// StopPath, asked with POST, ends the server that serves the site (when one
// set OnStop). It is how a lab that runs elsewhere, on another machine, ends
// the server it started: a request arrives where a closed pipe may not.
const StopPath = "/__lab/stop"

var images = map[string][]byte{}

func init() {
	for name, w := range map[string]int{"large": 1200, "small": 680} {
		images[name] = photo(w, w*3/4)
	}
}

// photo is a photo-like JPEG: smooth colour fields with fine grain, which
// compresses about as a real photo does.
func photo(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(int64(w)))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			n := uint8(r.Intn(28))
			img.Set(x, y, color.RGBA{uint8(60 + x*120/w + int(n)), uint8(70 + y*100/h + int(n)), uint8(90 + (x+y)*60/(w+h)), 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82})
	return buf.Bytes()
}

func (s *Site) serveImage(w http.ResponseWriter, r *http.Request) {
	size := "large"
	if r.URL.Query().Get("name") == "small" {
		size = "small"
	}
	s.imageBytes.Add(int64(len(images[size])))
	s.imageCount.Add(1)
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(images[size])
}

// stats is what StatsPath reports: count, then bytes, in that order.
type stats struct {
	Count int64 `json:"count"`
	Bytes int64 `json:"bytes"`
}

func (s *Site) serveStats(w http.ResponseWriter, _ *http.Request) {
	count, bytes := s.Images()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats{Count: count, Bytes: bytes})
}

func (s *Site) serveStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "POST to stop the server", http.StatusMethodNotAllowed)
		return
	}
	if s.stop != nil {
		s.stop()
	}
	w.WriteHeader(http.StatusNoContent)
}
