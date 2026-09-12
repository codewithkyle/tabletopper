package room

import (
	"context"

	"github.com/oklog/ulid/v2"
)

type Music struct {
	TrackID *ulid.ULID `json:"trackId"`
	Name    string     `json:"name"`
	Playing bool       `json:"playing"`
	Loop    bool       `json:"loop"`
	At      int        `json:"at"`
	Since   int64      `json:"since"`
}

func (m Music) Elapsed(now int64) int {
	if !m.Playing || m.Since == 0 {
		return m.At
	}
	return m.At + int(max(now-m.Since, 0))
}

type MusicUpdated struct {
	Kind
	Music Music `json:"music"`
}

func (*MusicUpdated) changeType() string { return "music.updated" }

type TrackInfo struct {
	Name string
}
type MusicLoad struct {
	AssetID ulid.ULID  `json:"assetId"`
	Track   *TrackInfo `json:"-"`
}

func (c *MusicLoad) Authorize(s *State, a Actor) error {
	return requireGM(a, "put music on")
}
func (c *MusicLoad) Resolve(ctx context.Context, lib Library, s *State) error {
	info, err := lib.Track(ctx, c.AssetID)
	if err != nil {
		return err
	}
	c.Track = &info
	return nil
}
func (c *MusicLoad) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if c.Track == nil {
		return nil, notFound("Track gone", "That track is no longer in your library.")
	}
	if err := checkRequiredName("track", c.Track.Name); err != nil {
		return nil, err
	}
	id := c.AssetID
	s.Music = Music{
		TrackID: &id,
		Name:    c.Track.Name,
		Playing: true,
		Loop:    s.Music.Loop,
		Since:   env.now(),
	}
	return nil, nil
}

type MusicPlay struct{}

func (c *MusicPlay) Authorize(s *State, a Actor) error {
	return requireGM(a, "start the music")
}
func (c *MusicPlay) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if s.Music.TrackID == nil {
		return nil, notFound("No track", "Pick a track from your library before starting it.")
	}
	if s.Music.Playing {
		return nil, nil
	}
	s.Music.Playing = true
	s.Music.Since = env.now()
	return nil, nil
}

type MusicPause struct{}

func (c *MusicPause) Authorize(s *State, a Actor) error {
	return requireGM(a, "pause the music")
}
func (c *MusicPause) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if !s.Music.Playing {
		return nil, nil
	}
	s.Music.At = s.Music.Elapsed(env.now())
	s.Music.Playing = false
	s.Music.Since = 0
	return nil, nil
}

type MusicStop struct{}

func (c *MusicStop) Authorize(s *State, a Actor) error {
	return requireGM(a, "stop the music")
}
func (c *MusicStop) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	s.Music.Playing = false
	s.Music.At = 0
	s.Music.Since = 0
	return nil, nil
}

type MusicSetLoop struct {
	Loop bool `json:"loop"`
}

func (c *MusicSetLoop) Authorize(s *State, a Actor) error {
	return requireGM(a, "set the music to repeat")
}
func (c *MusicSetLoop) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	s.Music.Loop = c.Loop
	return nil, nil
}

type MusicEnded struct {
	TrackID ulid.ULID `json:"trackId"`
}

func (c *MusicEnded) Authorize(s *State, a Actor) error {
	return requireGM(a, "end a track")
}
func (c *MusicEnded) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	m := s.Music
	if m.TrackID == nil || *m.TrackID != c.TrackID || !m.Playing || m.Loop {
		return nil, nil
	}
	s.Music.Playing = false
	s.Music.At = 0
	s.Music.Since = 0
	return nil, nil
}
func CloneMusic(m Music) Music {
	m.TrackID = cloneID(m.TrackID)
	return m
}
