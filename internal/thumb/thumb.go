// Package thumb makes the small still the catalog shows for an entry's
// linked image, GIF, YouTube or Vimeo video (HIG §1.7: nothing plays, or
// loads, until tapped). The server fetches the media once, keeps its first
// frame shrunk to card size as a JPEG (db.PutThumb), and serves that; a
// reader's browser asks the media's own host for nothing until the tap.
//
// The server fetches links the author pasted, so the fetch is guarded
// against server-side request forgery: every connection is checked against
// the address it actually reaches (not only the name), downloads are capped,
// and an image too large to decode safely is refused before decoding.
package thumb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // the first frame of a GIF
	"image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Width is the widest a thumbnail is made: X's "small", sharp on a card.
const Width = 680

const (
	maxDownload = 12 << 20 // a GIF can be large; its first frame is all we keep
	maxPixels   = 40e6     // refuse to decode anything bigger (decompression bombs)
	fetchTime   = 12 * time.Second
)

// Errors.
var (
	ErrNoStill  = errors.New("thumb: this link has no still to make")
	ErrBlocked  = errors.New("thumb: the address is not public")
	ErrTooLarge = errors.New("thumb: the media is too large")
)

var (
	youtubeID = regexp.MustCompile(`^[\w-]{6,}$`)
	vimeoID   = regexp.MustCompile(`^\d+$`)
	imageFile = regexp.MustCompile(`(?i)\.(gif|jpe?g|png|webp)$`)
)

// Source is where the still for a media link comes from, or "" if there is
// none to make (a video file: no decoder; a post on X: its snapshot has its
// own images; anything else). Vimeo is resolved through its oEmbed API.
func Source(mediaURL string) (kind, src string) {
	u, err := url.Parse(mediaURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	segments := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })
	last := ""
	if len(segments) > 0 {
		last = segments[len(segments)-1]
	}
	switch host {
	case "youtu.be", "youtube.com", "m.youtube.com":
		id := last
		if host != "youtu.be" && u.Path == "/watch" {
			id = u.Query().Get("v")
		}
		if youtubeID.MatchString(id) {
			return "video", "https://i.ytimg.com/vi/" + url.PathEscape(id) + "/hqdefault.jpg"
		}
		return "", ""
	case "vimeo.com", "player.vimeo.com":
		if vimeoID.MatchString(last) {
			return "video", "vimeo:" + last
		}
		return "", ""
	}
	if m := imageFile.FindStringSubmatch(strings.ToLower(u.Path)); m != nil {
		if m[1] == "gif" {
			return "gif", mediaURL
		}
		return "image", mediaURL
	}
	return "", ""
}

// Maker fetches media and makes stills.
type Maker struct {
	client *http.Client
}

// NewMaker returns a Maker whose connections reach public addresses only.
func NewMaker() *Maker {
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: publicOnly}
	transport := &http.Transport{
		DialContext:            dialer.DialContext,
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  8 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
	}
	return &Maker{client: &http.Client{
		Transport: transport,
		Timeout:   fetchTime,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("thumb: too many redirects")
			}
			return nil
		},
	}}
}

// publicOnly refuses a connection to any address that is not public: the
// check is on the resolved IP the socket is about to reach, so a public name
// that resolves to an internal address is refused too.
func publicOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return ErrBlocked
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return ErrBlocked
	}
	// Carrier-grade NAT (100.64.0.0/10) is not public either.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64 {
		return ErrBlocked
	}
	return nil
}

// Make returns a JPEG still for mediaURL, at most Width wide, and its size.
func (m *Maker) Make(ctx context.Context, mediaURL string) (jpg []byte, width, height int, err error) {
	_, src := Source(mediaURL)
	if src == "" {
		return nil, 0, 0, ErrNoStill
	}
	if id, ok := strings.CutPrefix(src, "vimeo:"); ok {
		if src, err = m.vimeoThumb(ctx, id); err != nil {
			return nil, 0, 0, err
		}
	}
	body, err := m.get(ctx, src, "image/")
	if err != nil {
		return nil, 0, 0, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("thumb: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || float64(cfg.Width)*float64(cfg.Height) > maxPixels {
		return nil, 0, 0, ErrTooLarge
	}
	img, _, err := image.Decode(bytes.NewReader(body)) // a GIF decodes to its first frame
	if err != nil {
		return nil, 0, 0, fmt.Errorf("thumb: %w", err)
	}
	return encode(img)
}

// encode shrinks img to at most Width wide (never enlarging), lays it on the
// page colour (a transparent GIF or PNG has no colour of its own to show),
// and writes a JPEG.
func encode(img image.Image) ([]byte, int, int, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > Width {
		h = h * Width / w
		w = Width
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.RGBA{0xf5, 0xf4, 0xf0, 0xff}}, image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 80}); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), w, h, nil
}

// get fetches src, at most maxDownload bytes, whose Content-Type starts
// with wantType.
func (m *Maker) get(ctx context.Context, src, wantType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "res-imperii-thumbnailer/1")
	res, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("thumb: %s answered %d", req.URL.Host, res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, wantType) {
		return nil, fmt.Errorf("thumb: %s is %q, not %s", req.URL.Host, ct, wantType)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxDownload+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDownload {
		return nil, ErrTooLarge
	}
	return body, nil
}

// vimeoThumb asks Vimeo's oEmbed API for a video's thumbnail address.
func (m *Maker) vimeoThumb(ctx context.Context, id string) (string, error) {
	body, err := m.get(ctx, "https://vimeo.com/api/oembed.json?url="+url.QueryEscape("https://vimeo.com/"+id), "application/json")
	if err != nil {
		return "", err
	}
	var o struct {
		ThumbnailURL string `json:"thumbnail_url"`
	}
	if err := json.Unmarshal(body, &o); err != nil || !strings.HasPrefix(o.ThumbnailURL, "https://") {
		return "", ErrNoStill
	}
	return o.ThumbnailURL, nil
}
