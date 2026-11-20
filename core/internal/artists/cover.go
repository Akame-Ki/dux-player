package artists

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Conava/dux-player/core/internal/atomicfile"
)

// lastfmNoCover is the star Last.fm serves for an album without art.
const lastfmNoCover = "2a96cbd8b46e442fc41c2b86b821562f"

// maxCoverURL keeps a found URL within what Discord takes for an image.
const maxCoverURL = 256

type coverEntry struct {
	URL string `json:"url,omitempty"`
	At  int64  `json:"at"`
}

// CoverURL is a public https URL of the album's front cover, "" when none matches. It sends artist and album to
// Last.fm, then MusicBrainz and the Cover Art Archive; answers are cached like the artist lookups.
func (s *Service) CoverURL(ctx context.Context, artist, album string) (string, error) {
	artist, album = strings.TrimSpace(artist), strings.TrimSpace(album)
	if artist == "" || album == "" {
		return "", nil
	}
	return s.online.cover(ctx, artist, album)
}

func (o *online) cover(ctx context.Context, artist, album string) (string, error) {
	file := filepath.Join(o.dir, "cover-"+cacheKey(artist+"\x00"+album)+".json")
	var e coverEntry
	if data, err := os.ReadFile(file); err == nil {
		_ = json.Unmarshal(data, &e)
	}
	if fresh(e.At, e.URL != "", o.now()) {
		return e.URL, nil
	}
	select {
	case o.sem <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-o.sem }()
	u, err := o.lastfmCover(ctx, artist, album)
	if err == nil && u == "" {
		u, err = o.coverArtArchive(ctx, artist, album)
	}
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(u, "https://") || len(u) > maxCoverURL {
		u = ""
	}
	data, err := json.Marshal(coverEntry{URL: u, At: o.now().Unix()})
	if err != nil {
		return "", err
	}
	return u, atomicfile.WriteFile(file, data, 0o600)
}

func (o *online) lastfmCover(ctx context.Context, artist, album string) (string, error) {
	key := o.keys(ProviderLastFM)
	if key == "" {
		return "", nil
	}
	q := url.Values{"method": {"album.getinfo"}, "artist": {artist}, "album": {album}, "api_key": {key}, "format": {"json"}}
	var r struct {
		Album struct {
			Name   string `json:"name"`
			Artist string `json:"artist"`
			Image  []struct {
				URL  string `json:"#text"`
				Size string `json:"size"`
			} `json:"image"`
		} `json:"album"`
	}
	if err := o.getJSON(ctx, o.ep.LastFM+"/2.0/?"+q.Encode(), &r); err != nil {
		return "", err
	}
	// a wrong cover is worse than the logo
	if !strings.EqualFold(r.Album.Name, album) || !strings.EqualFold(r.Album.Artist, artist) {
		return "", nil
	}
	for _, img := range r.Album.Image {
		if img.Size == "extralarge" && img.URL != "" && !strings.Contains(img.URL, lastfmNoCover) {
			return img.URL, nil
		}
	}
	return "", nil
}

func (o *online) coverArtArchive(ctx context.Context, artist, album string) (string, error) {
	if err := o.waitMusicBrainz(ctx); err != nil {
		return "", err
	}
	var r struct {
		Groups []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Score  int    `json:"score"`
			Credit []struct {
				Name string `json:"name"`
			} `json:"artist-credit"`
		} `json:"release-groups"`
	}
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, ``) + `"` }
	q := url.Values{"query": {"releasegroup:" + quote(album) + " AND artist:" + quote(artist)}, "fmt": {"json"}, "limit": {"3"}}
	if err := o.getJSON(ctx, o.ep.MusicBrainz+"/ws/2/release-group/?"+q.Encode(), &r); err != nil {
		return "", err
	}
	for _, g := range r.Groups {
		if g.Score < 90 || !strings.EqualFold(g.Title, album) || len(g.Credit) == 0 || !strings.EqualFold(g.Credit[0].Name, artist) {
			continue
		}
		var art struct {
			Images []struct {
				Front      bool              `json:"front"`
				Thumbnails map[string]string `json:"thumbnails"`
			} `json:"images"`
		}
		if err := o.getJSON(ctx, o.ep.CoverArt+"/release-group/"+url.PathEscape(g.ID), &art); err != nil {
			return "", err
		}
		for _, img := range art.Images {
			if u := img.Thumbnails["250"]; img.Front && u != "" {
				// the archive lists its own URLs as http
				return "https://" + strings.TrimPrefix(strings.TrimPrefix(u, "http://"), "https://"), nil
			}
		}
		return "", nil
	}
	return "", nil
}
