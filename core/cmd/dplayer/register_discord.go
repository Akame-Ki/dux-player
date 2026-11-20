package main

import (
	"context"
	"sync"
	"time"

	"github.com/duxlabs-dev/dux-go/log"

	"github.com/Conava/dux-player/core/internal/discord"
	"github.com/Conava/dux-player/core/internal/ipc"
	"github.com/Conava/dux-player/core/internal/player"
	"github.com/Conava/dux-player/core/internal/settings"
	"github.com/Conava/dux-player/core/internal/template"
)

const (
	// discordSeekDrift is how far playback may stray from the start time Discord shows before the card is sent again.
	discordSeekDrift = 2 * time.Second
	coverLookupWait  = 20 * time.Second
)

// discordCover looks a public cover URL up, "" when none matches; nil until the artists feature started.
func discordCover(ctx context.Context, albumArtist, artist, album string) (string, error) {
	svc := artistService.Load()
	if svc == nil {
		return "", nil
	}
	if albumArtist == "" {
		albumArtist = artist
	}
	return svc.CoverURL(ctx, albumArtist, album)
}

// presence mirrors the playing track to Discord while the setting is on.
type presence struct {
	mu     sync.Mutex
	client *discord.Client
	closed bool
	last   player.Status
	start  time.Time        // start time of the card Discord shows; zero when none
	text   discord.Activity // the templates, in the fields they fill
	cover  bool

	// lookup finds a public cover URL; it runs without mu.
	lookup   func(ctx context.Context, albumArtist, artist, album string) (string, error)
	image    string // cover URL for imageFor
	imageFor string // track key the image belongs to; set once its lookup ended
	looking  string // track key being looked up

	// libraryName reads a library's name; it runs without mu.
	libraryName func(id string) string
	library     string // name of the last status' library
	// nameID and name cache libraryName; only status() touches them, on the bus goroutine
	nameID, name string
}

// apply creates or closes the client to match the settings and resends the card when its templates changed.
func (p *presence) apply(s settings.Settings) {
	p.mu.Lock()
	var old *discord.Client
	text := discord.Activity{Details: s.DiscordDetails, State: s.DiscordState, LargeText: s.DiscordLargeText}
	changed := text != p.text || s.DiscordCover != p.cover
	p.text, p.cover = text, s.DiscordCover
	on := s.DiscordPresence
	switch {
	case on && p.client == nil && !p.closed:
		p.client = discord.New()
		p.show(p.last.Position)
	case !on && p.client != nil:
		old, p.client, p.start = p.client, nil, time.Time{}
	case changed && !p.start.IsZero():
		p.send()
	}
	p.mu.Unlock()
	if old != nil {
		old.Close()
	}
}

func (p *presence) status(st player.Status) {
	if st.Track != nil && st.Track.Library != p.nameID && p.libraryName != nil {
		p.nameID, p.name = st.Track.Library, p.libraryName(st.Track.Library)
	}
	p.mu.Lock()
	prev := p.last
	p.last, p.library = st, p.name
	if playing(prev) != playing(st) || playing(st) && !sameTrack(prev, st) {
		p.show(st.Position)
	}
	p.mu.Unlock()
}

func playing(st player.Status) bool { return st.State == player.StatePlaying && st.Track != nil }

func sameTrack(a, b player.Status) bool {
	return a.Track != nil && b.Track != nil && a.Track.Library == b.Track.Library && a.Track.ID == b.Track.ID
}

func trackKey(st player.Status) string {
	if st.Track == nil {
		return ""
	}
	return st.Track.Library + "\x00" + st.Track.ID
}

// position resends the card when a seek moved the implied start time.
func (p *presence) position(pos player.Position) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil || !pos.Playing || pos.Speed != 1 || p.start.IsZero() {
		return
	}
	start := time.UnixMilli(pos.At).Add(-time.Duration(pos.Position * float64(time.Second)))
	if d := start.Sub(p.start); d < discordSeekDrift && d > -discordSeekDrift {
		return
	}
	p.show(pos.Position)
}

// show sends or clears the card for the last status. Callers hold mu.
func (p *presence) show(position float64) {
	if p.client == nil {
		return
	}
	if !playing(p.last) {
		p.client.Clear()
		p.start = time.Time{}
		return
	}
	p.start = time.Now().Add(-time.Duration(position * float64(time.Second)))
	p.send()
}

// send sends the card with the current start time and starts a cover lookup the card lacks. Callers hold mu.
func (p *presence) send() {
	t := p.last.Track
	fields := template.TrackFields(*t)
	if p.library != "" {
		fields["library"] = p.library
	}
	p.client.Set(discord.Activity{
		Details:   template.Format(p.text.Details, fields),
		State:     template.Format(p.text.State, fields),
		LargeText: template.Format(p.text.LargeText, fields),
		Image:     p.shownImage(),
		Start:     p.start,
	})
	key := trackKey(p.last)
	if !p.cover || p.lookup == nil || p.imageFor == key || p.looking == key {
		return
	}
	p.looking = key
	go p.findCover(key, t.AlbumArtist, t.Artist, t.Album)
}

func (p *presence) findCover(key, albumArtist, artist, album string) {
	ctx, cancel := context.WithTimeout(context.Background(), coverLookupWait)
	defer cancel()
	url, err := p.lookup(ctx, albumArtist, artist, album)
	if err != nil {
		log.Infof("discord presence: cover lookup: %v", err)
	}
	p.mu.Lock()
	if p.looking == key {
		p.looking = ""
	}
	// a failed lookup counts as done for this track; the next track asks again
	p.image, p.imageFor = url, key
	if p.client != nil && !p.start.IsZero() && trackKey(p.last) == key {
		p.send()
	}
	p.mu.Unlock()
}

// shownImage is the cover the card shows, "" for the logo. Callers hold mu.
func (p *presence) shownImage() string {
	if !p.cover || p.imageFor != trackKey(p.last) {
		return ""
	}
	return p.image
}

func (p *presence) close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.apply(settings.Settings{})
}

func init() {
	p := &presence{}
	addFeature(feature{
		register: func(r *ipc.Registry, _ *daemon) {
			r.Group("discord", func(ctx context.Context, w *ipc.ConnWriter, req ipc.Request) {
				ctx, cancel := context.WithTimeout(ctx, coverLookupWait)
				defer cancel()
				p := req.Params
				url, err := discordCover(ctx, ipc.Str(p, "albumArtist"), ipc.Str(p, "artist"), ipc.Str(p, "album"))
				if err != nil {
					w.RespondError(req.ID, err.Error())
					return
				}
				w.Respond(req.ID, map[string]string{"image": url})
			}, ipc.MethodSpec{Name: "discord.cover", Desc: "The public cover URL the Discord card would show for an album, empty for the logo",
				Params: []ipc.ParamSpec{ipc.Opt("albumArtist", ""), ipc.Opt("artist", ""), ipc.Req("album", "")}, Read: true})
		},
		start: func(d *daemon) error {
			p.lookup = discordCover
			p.libraryName = func(id string) string {
				ctx, cancel := context.WithTimeout(d.Ctx, time.Second)
				defer cancel()
				lib, err := d.Libraries.Library(ctx, id)
				if err != nil {
					return ""
				}
				return lib.Name
			}
			p.apply(d.Settings.Get())
			d.Bus.On(d.Ctx, func(topic string, data any) {
				switch v := data.(type) {
				case player.Status:
					p.status(v)
				case player.Position:
					p.position(v)
				default:
					if topic == ipc.TopicSettings {
						p.apply(d.Settings.Get())
					}
				}
			}, ipc.TopicSettings, player.TopicStatus, player.TopicPosition)
			return nil
		},
		stop: p.close,
	})
}
