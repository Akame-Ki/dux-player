package jellyfin

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
	const info = `{"ServerName":"x","Version":"10.9.0","ProductName":"Jellyfin Server","Id":"abc"}`
	cases := []struct {
		name    string
		info    int
		body    string
		qc      int
		qcBody  string
		want    *bool
		wantErr error
	}{
		{"quick connect on", 200, info, 200, `true`, ptr(true), nil},
		{"quick connect off", 200, info, 200, `false`, ptr(false), nil},
		{"quick connect unreadable", 200, info, 500, ``, nil, nil},
		{"web page", 200, `<html></html>`, 200, `true`, nil, source.ErrNotRecognized},
		{"other json", 200, `{"subsonic-response":{}}`, 200, `true`, nil, source.ErrNotRecognized},
		{"not found", 404, `nope`, 200, `true`, nil, source.ErrNotRecognized},
		{"behind a login", 401, ``, 200, `true`, nil, source.ErrNotRecognized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				code, body := c.info, c.body
				if r.URL.Path == "/QuickConnect/Enabled" {
					code, body = c.qc, c.qcBody
				}
				w.WriteHeader(code)
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			cl, err := NewClient(srv.URL, "", "", "d", "t", nil)
			require.NoError(t, err)
			res, err := (&Source{client: cl}).Probe(context.Background())
			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, res.QuickConnect)
			assert.Equal(t, "Jellyfin Server", res.Type)
		})
	}
}

func TestProbeNotReached(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.URL
	srv.Close()
	cl, err := NewClient(addr, "", "", "d", "t", nil)
	require.NoError(t, err)
	_, err = (&Source{client: cl}).Probe(context.Background())
	require.Error(t, err)
	assert.NotErrorIs(t, err, source.ErrNotRecognized)
}

func ptr(b bool) *bool { return &b }
