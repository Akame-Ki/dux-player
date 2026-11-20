package subsonic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Conava/dux-player/core/internal/source"
)

func TestProbe(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		typ     string
		wantErr error
	}{
		{"subsonic", 200, `{"subsonic-response":{"status":"ok","version":"1.16.1","type":"navidrome","serverVersion":"0.5"}}`, "navidrome", nil},
		{"wrong password still proves it", 200, `{"subsonic-response":{"status":"failed","version":"1.16.1","error":{"code":40,"message":"Wrong username or password"}}}`, "", nil},
		{"web page", 200, `<html>hello</html>`, "", source.ErrNotRecognized},
		{"jellyfin json", 200, `{"ServerName":"x","Id":"1"}`, "", source.ErrNotRecognized},
		{"not found", 404, `nope`, "", source.ErrNotRecognized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var query string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query = r.URL.RawQuery
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			cl, err := NewClient(srv.URL, "", "", "", "", nil)
			require.NoError(t, err)
			res, err := (&Source{client: cl}).Probe(context.Background())
			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.typ, res.Type)
			assert.NotContains(t, query, "u=")
			assert.NotContains(t, query, "t=")
		})
	}
}

func TestProbeNotReached(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.URL
	srv.Close()
	cl, err := NewClient(addr, "", "", "", "", nil)
	require.NoError(t, err)
	_, err = (&Source{client: cl}).Probe(context.Background())
	require.Error(t, err)
	assert.NotErrorIs(t, err, source.ErrNotRecognized)
}
