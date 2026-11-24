package tweet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// syndicationURL is X's public endpoint behind its own embeds.
const syndicationURL = "https://cdn.syndication.twimg.com/tweet-result"

// maxResponse caps how much of X's response is read.
const maxResponse = 256 << 10

// token is the value X's embed script sends with each request: the id / 1e15
// times π, in base 36, without zeros or the point (as react-tweet computes it).
func token(id ID) string {
	n, err := strconv.ParseFloat(id.String(), 64)
	if err != nil {
		return "0"
	}
	return strings.NewReplacer("0", "", ".", "").Replace(formatBase36(n / 1e15 * math.Pi))
}

// formatBase36 writes a small positive float in base 36 exactly as V8's
// Number.prototype.toString(36) does (DoubleToRadixCString): fraction digits
// only while they still distinguish the value, rounded half to even.
func formatBase36(f float64) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	integer := math.Floor(f)
	fraction := f - integer
	delta := math.Max(0.5*(math.Nextafter(f, math.Inf(1))-f), math.Nextafter(0, 1))
	var frac []byte
	if fraction >= delta {
		for {
			fraction *= 36
			delta *= 36
			digit := int(fraction)
			frac = append(frac, chars[digit])
			fraction -= float64(digit)
			if fraction > 0.5 || (fraction == 0.5 && digit&1 == 1) {
				if fraction+delta > 1 {
					// Round up, carrying into earlier digits (or the integer).
					for {
						if len(frac) == 0 {
							integer++
							break
						}
						last := frac[len(frac)-1]
						frac = frac[:len(frac)-1]
						d := strings.IndexByte(chars, last)
						if d+1 < 36 {
							frac = append(frac, chars[d+1])
							break
						}
					}
					break
				}
			}
			if fraction < delta {
				break
			}
		}
	}
	s := strconv.FormatInt(int64(integer), 36)
	if len(frac) > 0 {
		s += "." + string(frac)
	}
	return s
}

// Fetch asks X for the tweet and decodes it. ErrUnavailable means X has no
// public post for the id.
func Fetch(ctx context.Context, client *http.Client, id ID) (Snapshot, error) {
	q := url.Values{"id": {id.String()}, "lang": {"en"}, "token": {token(id)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, syndicationURL+"?"+q.Encode(), nil)
	if err != nil {
		return Snapshot{}, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return Snapshot{}, err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return Snapshot{}, ErrUnavailable
	case res.StatusCode != http.StatusOK:
		return Snapshot{}, fmt.Errorf("tweet: syndication returned %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponse+1))
	if err != nil {
		return Snapshot{}, err
	}
	if len(body) > maxResponse {
		return Snapshot{}, errors.New("tweet: response too large")
	}
	if len(strings.TrimSpace(string(body))) == 0 || strings.TrimSpace(string(body)) == "{}" {
		return Snapshot{}, ErrUnavailable
	}
	return Decode(id, body)
}
