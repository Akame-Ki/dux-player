package artists

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverURL(t *testing.T) {
	var hits, down atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if down.Load() == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		q := r.URL.Query()
		switch r.URL.Path {
		case "/2.0/":
			img := map[string]string{"Hit": "https://lastfm.example/300x300/hit.jpg", "Star": "https://lastfm.example/300x300/" + lastfmNoCover + ".png"}[q.Get("album")]
			reply(map[string]any{"album": map[string]any{"name": q.Get("album"), "artist": "Band",
				"image": []any{map[string]any{"#text": img, "size": "extralarge"}}}})
		case "/ws/2/release-group/":
			reply(map[string]any{"release-groups": []any{
				map[string]any{"id": "rg-star", "title": "Star", "score": 100, "artist-credit": []any{map[string]any{"name": "Band"}}},
				map[string]any{"id": "rg-near", "title": "Near Miss", "score": 100, "artist-credit": []any{map[string]any{"name": "Band"}}},
			}})
		case "/release-group/rg-star":
			reply(map[string]any{"images": []any{map[string]any{"front": true, "thumbnails": map[string]string{"250": "http://coverartarchive.org/release/x/1-250.jpg"}}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	s := New(Config{Secrets: noSecrets{}, CacheDir: t.TempDir()})
	s.online.keys = func(string) string { return "key" }
	s.online.mbGap = 0
	s.online.ep = Endpoints{LastFM: srv.URL, MusicBrainz: srv.URL, CoverArt: srv.URL}

	for _, c := range []struct{ name, artist, album, want string }{
		{"last.fm match", "Band", "Hit", "https://lastfm.example/300x300/hit.jpg"},
		{"last.fm star falls back to the archive", "Band", "Star", "https://coverartarchive.org/release/x/1-250.jpg"},
		{"another artist on last.fm", "Other", "Hit", ""},
		{"no same-named release group", "Band", "Near", ""},
		{"no album", "Band", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := s.CoverURL(context.Background(), c.artist, c.album)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}

	t.Run("answers are cached, failures are not", func(t *testing.T) {
		before := hits.Load()
		got, err := s.CoverURL(context.Background(), "Band", "Hit")
		require.NoError(t, err)
		assert.Equal(t, "https://lastfm.example/300x300/hit.jpg", got)
		assert.Equal(t, before, hits.Load())

		down.Store(1)
		_, err = s.CoverURL(context.Background(), "Band", "Fresh")
		require.Error(t, err)
		down.Store(0)
		got, err = s.CoverURL(context.Background(), "Band", "Fresh")
		require.NoError(t, err)
		assert.Equal(t, "", got)
		assert.Greater(t, hits.Load(), before+1)
	})
}
