package player

// Track is a station the player plays: live, from wherever it is now, so
// there is no position to keep and nothing to play again.
type Track struct {
	ID        int64
	StationID string `json:"stationID,omitempty"` // Radio-Browser UUID; empty for built-in stations
	Name      string
	Detail    string // where it is, the language it speaks and what it plays
	Cover     string // the station's logo
	Stream    string // the live stream
	Codec     string
	Bitrate   int
}

// Session is where listening was left: the stations queued, and which of
// them plays.
type Session struct {
	Stations []Track `json:"stations,omitempty"`
	At       int     `json:"at"`
}

// Session returns where listening is, or false if nothing is queued.
func (p *Player) Session() (Session, bool) {
	if len(p.queue) == 0 || p.at < 0 {
		return Session{}, false
	}
	return Session{Stations: p.queue, At: p.at}, true
}

// Restore picks up where listening was left, paused: the station shows,
// and connects once the listener plays it.
func (p *Player) Restore(s Session) {
	if s.At < 0 || s.At >= len(s.Stations) {
		return
	}
	p.queue = append([]Track(nil), s.Stations...)
	p.at = s.At
	t := p.queue[s.At]
	p.track = &t
	p.playing = false
	p.announce(&t, nil)
	p.sync()
}
