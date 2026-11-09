package radio

import (
	"context"
	"strings"
)

// The directory's tags are flat: "music" does not include these genres.
// Use the same OR grouping for directory results and offline stations.
var musicTags = []string{
	"music", "pop", "classical", "jazz", "rock", "electronic", "ambient",
	"blues", "country", "folk", "soul", "hip hop", "rap", "rnb", "metal",
	"dance", "lofi", "chillout", "reggae", "oldies", "soundtrack",
}

// findMusicPage scans popularity-ordered pages until it has enough music.
// A batch query replaces the old per-genre fan-out without narrowing the
// genre set or filtering only the first page of results.
func findMusicPage(parent context.Context, f Filter, limit, offset int) (Page, error) {
	ctx, cancel := context.WithTimeout(parent, getTimeout)
	defer cancel()
	f.Tag = ""
	pageSize := max(3*limit, 60)
	stations := make([]Station, 0, limit)
	seen := make(map[string]bool)
	for {
		page, err := findPage(ctx, f, pageSize, offset)
		if err != nil {
			// A failed continuation is not a successful, complete result.
			return Page{}, err
		}
		for i, st := range page.stations {
			if seen[st.ID] || !matchesContentTag(st.Tags, "music") {
				continue
			}
			seen[st.ID] = true
			stations = append(stations, st)
			if len(stations) == limit {
				next := offset + page.indices[i] + 1
				return Page{Stations: stations, NextOffset: next, HasMore: next < offset+page.count || page.count >= pageSize}, nil
			}
		}
		if page.count < pageSize {
			return Page{Stations: stations, NextOffset: offset + page.count}, nil
		}
		offset += page.count
	}
}

func matchesContentTag(tags, selected string) bool {
	if !strings.EqualFold(selected, "music") {
		return containsValue(tags, selected)
	}
	for _, tag := range musicTags {
		if containsValue(tags, tag) {
			return true
		}
	}
	return false
}

// containsValue reports whether list, comma-separated, holds value, ignoring
// case and the space around either.
func containsValue(list, value string) bool {
	if value == "" {
		return true
	}
	for _, item := range strings.Split(list, ",") {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return true
		}
	}
	return false
}
