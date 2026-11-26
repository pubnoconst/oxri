package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Strictly read-only prototype server: reads TURSO_DATABASE_URL & TURSO_AUTH_TOKEN
// from ../oxri-parent/db.env and runs ONLY SELECT queries over Turso's HTTP pipeline.
// Never mutates the database and never logs secrets.

type tursoClient struct {
	endpoint string
	token    string
	http     *http.Client
}

type hranaReq struct {
	Requests []hranaStep `json:"requests"`
}

type hranaStep struct {
	Type string     `json:"type"`
	Stmt *hranaStmt `json:"stmt,omitempty"`
}

type hranaStmt struct {
	SQL  string     `json:"sql"`
	Args []hranaArg `json:"args,omitempty"`
}

type hranaArg struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type hranaResp struct {
	Results []struct {
		Type  string `json:"type"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
		Response *struct {
			Type   string `json:"type"`
			Result *struct {
				Rows [][]hranaVal `json:"rows"`
			} `json:"result,omitempty"`
		} `json:"response,omitempty"`
	} `json:"results"`
}

type hranaVal struct {
	Type   string          `json:"type"`
	Value  json.RawMessage `json:"value,omitempty"`
	Base64 string          `json:"base64,omitempty"`
}

func (v hranaVal) str() string {
	if v.Type == "null" || len(v.Value) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(v.Value, &s); err == nil {
		return s
	}
	return strings.Trim(string(v.Value), `"`)
}

func (v hranaVal) i64() int64 {
	s := v.str()
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func (v hranaVal) blob() []byte {
	if v.Type != "blob" || v.Base64 == "" {
		return nil
	}
	if b, err := base64.StdEncoding.DecodeString(v.Base64); err == nil {
		return b
	}
	b, _ := base64.RawStdEncoding.DecodeString(v.Base64)
	return b
}

func newTursoClient(repoRoot string) (*tursoClient, error) {
	envCandidates := []string{
		filepath.Join(repoRoot, "..", "oxri-parent", "db.env"),
		filepath.Join(repoRoot, "..", "db.env"),
	}
	vars := map[string]string{}
	for _, p := range envCandidates {
		readEnvInto(p, vars)
	}
	dbURL := os.Getenv("TURSO_DATABASE_URL")
	if dbURL == "" {
		dbURL = vars["TURSO_DATABASE_URL"]
	}
	token := os.Getenv("TURSO_AUTH_TOKEN")
	if token == "" {
		token = vars["TURSO_AUTH_TOKEN"]
	}
	if dbURL == "" {
		return nil, errors.New("TURSO_DATABASE_URL not found in environment or ../oxri-parent/db.env")
	}
	u, err := url.Parse(strings.TrimSpace(dbURL))
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "libsql", "wss":
		u.Scheme = "https"
	case "ws":
		u.Scheme = "http"
	}
	u.Path = "/v2/pipeline"
	u.RawQuery = ""
	return &tursoClient{
		endpoint: u.String(),
		token:    token,
		http:     &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func readEnvInto(path string, dst map[string]string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\uFEFF"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if _, exists := dst[k]; !exists && k != "" {
			dst[k] = v
		}
	}
}

func (c *tursoClient) queryBatch(stmts ...hranaStmt) ([][][]hranaVal, error) {
	steps := make([]hranaStep, 0, len(stmts)+1)
	for i := range stmts {
		stmt := stmts[i]
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt.SQL)), "SELECT ") {
			return nil, errors.New("prototype server only allows SELECT statements")
		}
		steps = append(steps, hranaStep{Type: "execute", Stmt: &stmt})
	}
	steps = append(steps, hranaStep{Type: "close"})

	payload, err := json.Marshal(hranaReq{Requests: steps})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("turso HTTP %d", resp.StatusCode)
	}
	var parsed hranaResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([][][]hranaVal, 0, len(stmts))
	for i := 0; i < len(stmts) && i < len(parsed.Results); i++ {
		r := parsed.Results[i]
		if r.Type == "error" && r.Error != nil {
			return nil, errors.New(r.Error.Message)
		}
		if r.Response != nil && r.Response.Result != nil {
			out = append(out, r.Response.Result.Rows)
		} else {
			out = append(out, nil)
		}
	}
	return out, nil
}

// Stored JSON shapes matching internal/tweet and internal/domain
type tweetMedia struct {
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Width  int    `json:"w"`
	Height int    `json:"h"`
}

type tweetQuoted struct {
	Name   string `json:"name"`
	Handle string `json:"handle"`
	Text   string `json:"text"`
}

type tweetSnapshot struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Handle    string       `json:"handle"`
	AvatarURL string       `json:"avatar,omitempty"`
	Created   time.Time    `json:"created"`
	Text      string       `json:"text"`
	Media     []tweetMedia `json:"media,omitempty"`
	Quoted    *tweetQuoted `json:"quoted,omitempty"`
}

type storedPassage struct {
	Reference string `json:"ref"`
	Version   string `json:"ver"`
	Text      string `json:"text"`
}

// API DTOs for the throwaway browser
type apiTweet struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Handle      string       `json:"handle"`
	AvatarURL   string       `json:"avatarUrl"`
	PostURL     string       `json:"postUrl"`
	CreatedDate string       `json:"createdDate"`
	Text        string       `json:"text"`
	CharCount   int          `json:"charCount"`
	Archetype   string       `json:"archetype"` // "caption" | "snippet" | "statement"
	Media       []tweetMedia `json:"media,omitempty"`
	Quoted      *tweetQuoted `json:"quoted,omitempty"`
}

type apiThumb struct {
	URL    string `json:"url"`
	Kind   string `json:"kind"`
	Width  int    `json:"w"`
	Height int    `json:"h"`
}

type apiEntry struct {
	ID         int64          `json:"id"`
	Date       string         `json:"date"`
	Published  bool           `json:"published"`
	Bookmarked bool           `json:"bookmarked"`
	Tags       []string       `json:"tags"`
	Excerpt    string         `json:"excerpt"`
	MediaURL   string         `json:"mediaUrl"`
	MediaHost  string         `json:"mediaHost"`
	Tweet      *apiTweet      `json:"tweet,omitempty"`
	Passage    *storedPassage `json:"passage,omitempty"`
	Thumb      *apiThumb      `json:"thumb,omitempty"`
}

var (
	statusPathRe = regexp.MustCompile(`^/([A-Za-z0-9_]{1,15}|i(?:/web)?)/status(?:es)?/([0-9]{1,20})(?:/|$)`)
	mdLinkRe     = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	mdDecorRe    = regexp.MustCompile("(\\*\\*|\\*|__|_|`|~|^#+\\s+|^>\\s+)")
	blankLinesRe = regexp.MustCompile(`\n{2,}`)
	thumbKeyRe   = regexp.MustCompile(`^[0-9a-f]{24}$`)
)

func parseTweetID(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	switch strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") {
	case "x.com", "twitter.com", "mobile.x.com", "mobile.twitter.com":
	default:
		return ""
	}
	m := statusPathRe.FindStringSubmatch(u.Path)
	if m == nil {
		return ""
	}
	return m[2]
}

func classifyThumbKind(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host == "youtube.com" || host == "youtu.be" || host == "m.youtube.com" || host == "vimeo.com" {
		return "video"
	}
	p := strings.ToLower(u.Path)
	switch {
	case strings.HasSuffix(p, ".gif"):
		return "gif"
	case strings.HasSuffix(p, ".mp4"), strings.HasSuffix(p, ".webm"), strings.HasSuffix(p, ".mov"):
		return "video"
	case strings.HasSuffix(p, ".jpg"), strings.HasSuffix(p, ".jpeg"), strings.HasSuffix(p, ".png"), strings.HasSuffix(p, ".webp"), strings.HasSuffix(p, ".avif"):
		return "image"
	}
	return "image"
}

func makeExcerpt(md string) string {
	s := strings.ReplaceAll(md, "\r\n", "\n")
	s = mdLinkRe.ReplaceAllString(s, "$1")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		l = strings.TrimSpace(l)
		l = strings.TrimPrefix(l, "> ")
		l = strings.TrimLeft(l, "#")
		l = strings.TrimSpace(l)
		l = strings.ReplaceAll(l, "**", "")
		l = strings.ReplaceAll(l, "__", "")
		l = strings.Trim(l, "*_`")
		lines[i] = l
	}
	s = strings.TrimSpace(strings.Join(lines, "\n"))
	s = blankLinesRe.ReplaceAllString(s, "\n")
	s = html.UnescapeString(s)
	if utf8.RuneCountInString(s) > 280 {
		r := []rune(s)
		s = strings.TrimSpace(string(r[:280])) + "…"
	}
	return s
}

func proxyXImageURL(raw string, small bool) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Hostname() != "pbs.twimg.com" {
		return ""
	}
	if small {
		q := u.Query()
		q.Set("name", "small")
		u.RawQuery = q.Encode()
	}
	return "/ximg?u=" + url.QueryEscape(u.String())
}

type imgBlob struct {
	contentType string
	data        []byte
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8788", "listen address")
	dir := flag.String("dir", ".", "prototype directory")
	repo := flag.String("repo", `c:\Users\relay\proj\oxri`, "oxri repository root")
	flag.Parse()

	tc, err := newTursoClient(*repo)
	if err != nil {
		log.Fatalf("Failed to configure read-only Turso client: %v", err)
	}

	var (
		cacheMu      sync.Mutex
		cachedAt     time.Time
		cachedResult []apiEntry

		thumbMu    sync.Mutex
		thumbCache = map[string][]byte{}

		ximgMu    sync.Mutex
		ximgCache = map[string]imgBlob{}
	)

	loadRealEntries := func(force bool) ([]apiEntry, error) {
		cacheMu.Lock()
		defer cacheMu.Unlock()
		if !force && cachedResult != nil && time.Since(cachedAt) < 2*time.Minute {
			return cachedResult, nil
		}

		res, err := tc.queryBatch(
			hranaStmt{SQL: `SELECT id, created_at, media_url, COALESCE(tag1,''), COALESCE(tag2,''), COALESCE(tag3,''), body, bookmarked, published, COALESCE(passage,'') FROM journals ORDER BY created_at DESC, id DESC`},
			hranaStmt{SQL: `SELECT id, body FROM tweet_snapshots WHERE status = 'ok'`},
			hranaStmt{SQL: `SELECT key, url, width, height FROM media_thumbs WHERE status = 'ok'`},
		)
		if err != nil {
			return nil, err
		}

		tweetsByID := map[string]tweetSnapshot{}
		if len(res) > 1 {
			for _, row := range res[1] {
				if len(row) < 2 {
					continue
				}
				var snap tweetSnapshot
				if json.Unmarshal([]byte(row[1].str()), &snap) == nil && snap.ID != "" {
					tweetsByID[row[0].str()] = snap
				}
			}
		}

		type thumbMeta struct {
			key  string
			w, h int
		}
		thumbsByURL := map[string]thumbMeta{}
		if len(res) > 2 {
			for _, row := range res[2] {
				if len(row) < 4 {
					continue
				}
				thumbsByURL[row[1].str()] = thumbMeta{
					key: row[0].str(),
					w:   int(row[2].i64()),
					h:   int(row[3].i64()),
				}
			}
		}

		var entries []apiEntry
		if len(res) > 0 {
			for _, row := range res[0] {
				if len(row) < 10 {
					continue
				}
				id := row[0].i64()
				createdMs := row[1].i64()
				if createdMs <= 0 {
					createdMs = id
				}
				mediaURL := row[2].str()
				var tags []string
				for _, idx := range []int{3, 4, 5} {
					if t := strings.TrimSpace(row[idx].str()); t != "" {
						tags = append(tags, t)
					}
				}
				body := row[6].str()
				bookmarked := row[7].i64() == 1
				published := row[8].i64() == 1
				passageRaw := strings.TrimSpace(row[9].str())

				var host string
				if u, pErr := url.Parse(mediaURL); pErr == nil {
					host = strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
				}

				item := apiEntry{
					ID:         id,
					Date:       time.UnixMilli(createdMs).Local().Format("Jan 2, 2006"),
					Published:  published,
					Bookmarked: bookmarked,
					Tags:       tags,
					Excerpt:    makeExcerpt(body),
					MediaURL:   mediaURL,
					MediaHost:  host,
				}

				if passageRaw != "" {
					var p storedPassage
					if json.Unmarshal([]byte(passageRaw), &p) == nil && p.Text != "" {
						item.Passage = &p
					}
				}

				if item.Passage == nil {
					if tid := parseTweetID(mediaURL); tid != "" {
						if snap, ok := tweetsByID[tid]; ok {
							trimmed := strings.TrimSpace(snap.Text)
							chars := utf8.RuneCountInString(trimmed)
							hasMedia := len(snap.Media) > 0
							arch := "statement"
							if hasMedia && chars <= 80 {
								arch = "caption"
							} else if hasMedia {
								arch = "snippet"
							}

							mediaOut := make([]tweetMedia, 0, len(snap.Media))
							for _, m := range snap.Media {
								if prox := proxyXImageURL(m.URL, true); prox != "" {
									mediaOut = append(mediaOut, tweetMedia{
										Kind:   m.Kind,
										URL:    prox,
										Width:  m.Width,
										Height: m.Height,
									})
								}
							}
							cDate := ""
							if !snap.Created.IsZero() {
								cDate = snap.Created.Format("Jan 2, 2006")
							}
							item.Tweet = &apiTweet{
								ID:          snap.ID,
								Name:        snap.Name,
								Handle:      snap.Handle,
								AvatarURL:   proxyXImageURL(snap.AvatarURL, false),
								PostURL:     "https://x.com/" + url.PathEscape(snap.Handle) + "/status/" + snap.ID,
								CreatedDate: cDate,
								Text:        trimmed,
								CharCount:   chars,
								Archetype:   arch,
								Media:       mediaOut,
								Quoted:      snap.Quoted,
							}
						}
					}
				}

				if item.Passage == nil && item.Tweet == nil {
					if tm, ok := thumbsByURL[mediaURL]; ok {
						item.Thumb = &apiThumb{
							URL:    "/thumb/" + tm.key + ".jpg",
							Kind:   classifyThumbKind(mediaURL),
							Width:  tm.w,
							Height: tm.h,
						}
					}
				}

				entries = append(entries, item)
			}
		}

		cachedResult = entries
		cachedAt = time.Now()
		return entries, nil
	}

	mux := http.NewServeMux()
	fontsDir := filepath.Join(*repo, "assets", "fonts")
	mux.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir(filepath.Clean(fontsDir)))))

	mux.HandleFunc("/api/entries", func(w http.ResponseWriter, r *http.Request) {
		force := r.URL.Query().Get("refresh") == "1"
		entries, err := loadRealEntries(force)
		if err != nil {
			http.Error(w, "Failed to read entries: "+err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"entries": entries,
		})
	})

	mux.HandleFunc("/thumb/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/thumb/")
		key := strings.TrimSuffix(name, ".jpg")
		if !thumbKeyRe.MatchString(key) {
			http.NotFound(w, r)
			return
		}
		thumbMu.Lock()
		cached, ok := thumbCache[key]
		thumbMu.Unlock()
		if !ok {
			sum := sha256.Sum256([]byte(key))
			_ = hex.EncodeToString(sum[:])
			res, err := tc.queryBatch(hranaStmt{
				SQL:  `SELECT body FROM media_thumbs WHERE key = ? AND status = 'ok' LIMIT 1`,
				Args: []hranaArg{{Type: "text", Value: key}},
			})
			if err != nil || len(res) == 0 || len(res[0]) == 0 || len(res[0][0]) == 0 {
				http.NotFound(w, r)
				return
			}
			cached = res[0][0][0].blob()
			if len(cached) == 0 {
				http.NotFound(w, r)
				return
			}
			thumbMu.Lock()
			thumbCache[key] = cached
			thumbMu.Unlock()
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(cached)
	})

	httpClient := &http.Client{Timeout: 12 * time.Second}
	mux.HandleFunc("/ximg", func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.Query().Get("u")
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Hostname() != "pbs.twimg.com" || u.User != nil || u.Port() != "" {
			http.NotFound(w, r)
			return
		}
		cleanURL := u.String()
		ximgMu.Lock()
		blob, ok := ximgCache[cleanURL]
		ximgMu.Unlock()
		if !ok {
			req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, cleanURL, nil)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			resp, err := httpClient.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				if resp != nil {
					resp.Body.Close()
				}
				http.NotFound(w, r)
				return
			}
			data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			resp.Body.Close()
			if err != nil || len(data) == 0 {
				http.NotFound(w, r)
				return
			}
			ct := resp.Header.Get("Content-Type")
			if ct == "" {
				ct = "image/jpeg"
			}
			blob = imgBlob{contentType: ct, data: data}
			ximgMu.Lock()
			ximgCache[cleanURL] = blob
			ximgMu.Unlock()
		}
		w.Header().Set("Content-Type", blob.contentType)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(blob.data)
	})

	mux.Handle("/", http.FileServer(http.Dir(filepath.Clean(*dir))))

	entries, warmErr := loadRealEntries(true)
	if warmErr != nil {
		log.Printf("Warning: initial Turso read failed: %v", warmErr)
	} else {
		log.Printf("Loaded %d real journal entries (read-only) for throwaway catalog on http://%s", len(entries), *addr)
	}
	log.Fatal(http.ListenAndServe(*addr, mux))
}
