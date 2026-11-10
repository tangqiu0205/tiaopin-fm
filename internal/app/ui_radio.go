package app

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/player"
	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// The stations page plays FM: what Radio-Browser's directory lists, or
// the built-in stations when no mirror answers, live.

// Content, location and language are independent API facets, not tabs.
var stationTags = []struct{ tag, label string }{
	{"", "全部内容"}, {"music", "音乐"}, {"news", "新闻"}, {"talk", "谈话"},
	{"sports", "体育"}, {"culture", "文化"}, {"entertainment", "娱乐"},
}

var stationCountries = []struct{ code, label string }{
	{"", "全部地区"}, {"CN", "中国"}, {"HK", "中国香港"}, {"TW", "中国台湾"},
	{"SG", "新加坡"}, {"US", "美国"}, {"GB", "英国"}, {"JP", "日本"},
	{"KR", "韩国"}, {"DE", "德国"}, {"FR", "法国"}, {"CA", "加拿大"}, {"AU", "澳大利亚"},
}

var stationLanguages = []struct{ value, label string }{
	{"", "全部语言"}, {"chinese", "中文"}, {"cantonese", "粤语"}, {"english", "英语"},
	{"japanese", "日语"}, {"korean", "韩语"}, {"german", "德语"},
	{"french", "法语"}, {"spanish", "西班牙语"},
}

// stationLimit is the maximum number of stations in each loaded batch.
const stationLimit = 60

var findStations = radio.FindPage

// radioState is the stations page: what it lists, and what it asked the
// directory for.
type radioState struct {
	tag          int    // which of stationTags, combined with the other facets
	country      string // selected region label; empty means all
	language     string // selected language label; empty means all
	query        string // search field draft
	name         string // submitted station name, combined with the facets
	asked        string // what is listed, or asked for: "" before the page shows
	loading      bool   // the list is asked for
	stale        bool   // expired local data remains visible during a refresh
	stations     []radio.Station
	err          string
	moreErr      string // continuation failures need an explicit retry
	nextOffset   int
	hasMore      bool
	generation   uint64
	cancel       context.CancelFunc
	cache        stationListCache // successful directory lists for recent filter combinations
	cacheLoading bool             // startup disk read finishes before directory requests
}

var countryNames = map[string]string{
	"CN": "中国", "HK": "中国香港", "TW": "中国台湾", "MO": "中国澳门",
	"SG": "新加坡", "MY": "马来西亚", "US": "美国", "CA": "加拿大",
	"GB": "英国", "IE": "爱尔兰", "FR": "法国", "DE": "德国",
	"IT": "意大利", "ES": "西班牙", "NL": "荷兰", "SE": "瑞典",
	"NO": "挪威", "DK": "丹麦", "FI": "芬兰", "PL": "波兰",
	"RU": "俄罗斯", "JP": "日本", "KR": "韩国", "IN": "印度",
	"AU": "澳大利亚", "NZ": "新西兰", "BR": "巴西", "AR": "阿根廷",
	"MX": "墨西哥", "GR": "希腊", "PT": "葡萄牙", "CH": "瑞士",
	"AT": "奥地利", "BE": "比利时", "TR": "土耳其", "ZA": "南非",
}

var languageNames = map[string]string{
	"chinese": "中文", "cantonese": "粤语", "english": "英语", "japanese": "日语",
	"korean": "韩语", "german": "德语", "french": "法语", "spanish": "西班牙语",
	"italian": "意大利语", "portuguese": "葡萄牙语", "russian": "俄语",
	"dutch": "荷兰语", "swedish": "瑞典语", "instrumental": "器乐",
	"various": "多语言", "music": "音乐", "news": "新闻", "talk": "谈话",
}

// countryName is where a station is, as the listener knows it.
func countryName(st radio.Station) string {
	if n, ok := countryNames[strings.ToUpper(st.CountryCode)]; ok {
		return n
	}
	if st.Country != "" {
		return st.Country
	}
	return "网络电台"
}

// languageName is what a station speaks, as the listener knows it.
func languageName(l string) string {
	for _, s := range strings.Split(l, ",") {
		if n, ok := languageNames[strings.ToLower(strings.TrimSpace(s))]; ok {
			return n
		}
	}
	return ""
}

// displayCodec is the codec a card shows. The directory answers "UNKNOWN"
// when it does not know one, and a stream's real format is only settled when
// it connects; a placeholder is shown as nothing rather than as a format.
func displayCodec(codec string) string {
	c := strings.TrimSpace(codec)
	if strings.EqualFold(c, "unknown") || c == "?" {
		return ""
	}
	return c
}

// stationMeta says how a station streams: its codec, and its bitrate.
func stationMeta(st radio.Station) string {
	var parts []string
	if l := languageName(st.Language); l != "" {
		parts = append(parts, l)
	}
	codec := displayCodec(st.Codec)
	switch {
	case codec != "" && st.Bitrate > 0:
		parts = append(parts, fmt.Sprintf("%s %d kbps", codec, st.Bitrate))
	case codec != "":
		parts = append(parts, codec)
	case st.Bitrate > 0:
		parts = append(parts, fmt.Sprintf("%d kbps", st.Bitrate))
	}
	return strings.Join(parts, " · ")
}

// stationLine says what a station is, for its row: where it is, and what
// it plays.
func stationLine(st radio.Station) string {
	parts := []string{countryName(st)}
	var tags []string
	for _, t := range strings.Split(st.Tags, ",") {
		if t = strings.TrimSpace(t); t != "" && len(tags) < 2 {
			tags = append(tags, t)
		}
	}
	if len(tags) > 0 {
		parts = append(parts, strings.Join(tags, " / "))
	}
	return strings.Join(parts, " · ")
}

// stationTrack plays a station as a track: its stream, live, with what
// the card and the player bar say of it.
func stationTrack(st radio.Station) player.Track {
	id := st.ID
	if st.Curated {
		id = ""
	}
	return player.Track{
		StationID: id,
		ID:        stationID(st.ID),
		Name:      st.Name,
		Detail:    stationMeta(st),
		Cover:     st.Icon,
		Stream:    st.Stream,
		Codec:     st.Codec,
		Bitrate:   st.Bitrate,
	}
}

// stationID is a station's ID as a track's: the same for the same
// station, and no song's.
func stationID(id string) int64 {
	h := fnv.New64a()
	h.Write([]byte(id))
	return int64(h.Sum64() & math.MaxInt64)
}

// stationPlaying reports whether st is the station playing.
func (a *app) stationPlaying(st radio.Station) bool {
	cur := a.pl.Current()
	return cur != nil && cur.ID == stationID(st.ID)
}

// playStation plays the station at i of the list, and the list with it:
// next changes station.
func (a *app) playStation(i int) {
	st := a.pageStations()
	if i < 0 || i >= len(st) {
		return
	}
	tracks := make([]player.Track, len(st))
	for k, s := range st {
		tracks[k] = stationTrack(s)
	}
	a.pl.Play(tracks, i)
	a.lib.played(st[i])
	if !st[i].Curated {
		// The directory counts the stations played, to sort them.
		go radio.Click(st[i].ID)
	}
}

// playRandom plays one of the stations the page lists, at random: one it
// can play, and not the one already playing, so that the dice moves.
func (a *app) playRandom() {
	st := a.pageStations()
	var pick []int
	for i, s := range st {
		if stationPlayable(s) && !a.stationPlaying(s) {
			pick = append(pick, i)
		}
	}
	if len(pick) == 0 {
		// A single station, or none but the one playing.
		for i, s := range st {
			if stationPlayable(s) {
				a.playStation(i)
				return
			}
		}
		return
	}
	a.playStation(pick[rand.IntN(len(pick))])
}

// stationFilter translates UI choices into independent API parameters.
func (a *app) stationFilter() radio.Filter {
	f := radio.Filter{Name: a.radio.name, Tag: stationTags[a.radio.tag].tag}
	for _, country := range stationCountries {
		if country.label == a.radio.country {
			f.CountryCode = country.code
			break
		}
	}
	for _, language := range stationLanguages {
		if language.label == a.radio.language {
			f.Language = language.value
			break
		}
	}
	return f
}

// restoreRadioFilters runs before the first view and directory request.
func (a *app) restoreRadioFilters() {
	a.radio.country, a.radio.language = stationCountries[0].label, stationLanguages[0].label
	for _, country := range stationCountries {
		if country.code == a.settings.RadioCountry {
			a.radio.country = country.label
			break
		}
	}
	for _, language := range stationLanguages {
		if language.value == a.settings.RadioLanguage {
			a.radio.language = language.label
			break
		}
	}
}

// saveRadioFilters persists dropdown changes immediately, including choosing
// all regions or languages, without remembering transient names or genres.
func (a *app) saveRadioFilters() {
	f := a.stationFilter()
	a.settings.RadioCountry, a.settings.RadioLanguage = f.CountryCode, f.Language
	if err := a.settings.save(); err != nil {
		a.toast = "无法保存电台筛选：" + err.Error()
	}
}

// stationKey names the complete set of directory filters.
func (a *app) stationKey() string {
	return fmt.Sprintf("%#v", a.stationFilter())
}

// loadStations reuses fresh lists and asks the directory only on a cache miss.
func (a *app) loadStations() {
	if a.radio.cacheLoading {
		return
	}
	key := a.stationKey()
	if a.radio.asked == key && a.radio.loading {
		return
	}
	if a.radio.cancel != nil {
		a.radio.cancel()
		a.radio.cancel = nil
	}
	a.radio.generation++
	page, fresh, cached := a.radio.cache.lookup(key, time.Now())
	sortStations(page.Stations)
	a.radio.asked, a.radio.err, a.radio.moreErr = key, "", ""
	a.radio.stations = appendUniqueStations(nil, page.Stations)
	a.radio.nextOffset, a.radio.hasMore = page.NextOffset, page.HasMore
	a.radio.loading, a.radio.stale = false, cached && !fresh
	if !fresh {
		a.requestStations(0)
	}
}

// pageStations is what the page lists: the directory's stations, the ones
// the listener favorited, or the ones they played lately.
func (a *app) pageStations() []radio.Station {
	switch a.page {
	case pageFavorites:
		return a.lib.Favorites
	case pageRecent:
		return a.lib.Recent
	}
	return a.radio.stations
}

// radioPage is what the page lists, the stations among them, in a grid of
// tiles.
func (a *app) radioPage(c *ui.Context) {
	top, bottom := a.insets(c)
	stations := a.pageStations()
	// Tiles out of view are not built, their marks with them: a list
	// scrolled away from its start or end has content under the bars.
	a.underTop = true
	width, _ := c.Size()
	cols := 3
	if width-sidebarWidth-2*pagePad < 3*260+2*stationGap {
		cols = 2
	}
	rows := (len(stations)+cols-1)/cols + 1
	paginating := a.page == pageRadio && a.radio.asked != "" && a.radio.err == "" &&
		!a.radio.stale && a.radio.nextOffset > 0
	// The header owns initial loading and stale-cache refreshes. Keep the
	// footer for continuation status, including errors after an empty batch.
	footer := paginating && (len(stations) > 0 || a.radio.moreErr != "")
	if footer {
		rows++
	}
	key := "stations"
	switch a.page {
	case pageFavorites:
		key = "favorites"
	case pageRecent:
		key = "recent"
	}
	ui.List(c.Key(key), &a.listScroll, rows, func(i int) {
		if i == 0 {
			ui.Column(c).Padding(0, 0, 6, 0).Children(func() {
				a.startMark(c)
				a.radioHeader(c)
			})
			return
		}
		if footer && i == rows-1 {
			a.stationListFooter(c)
			return
		}
		first := (i - 1) * cols
		last := min(first+cols, len(stations))
		ui.Row(c).Gap(stationGap).Margin(0, 0, stationGap, 0).Children(func() {
			for k := first; k < last; k++ {
				a.stationTile(c, stations[k], k)
			}
			// A last row short of tiles keeps the others their width.
			for k := last; k < first+cols; k++ {
				ui.Box(c).Grow(1).Basis(0).MinWidth(0).Padding(14).Border(1, ui.Transparent)
			}
		})
	}).Grow(1).Padding(top+pageGap, pagePad, bottom+24, pagePad).ScrollbarInsets(top, 0, bottom, 0)
	if paginating {
		_, last := a.listScroll.Visible()
		if last >= rows-4 {
			a.loadMoreStations()
		}
	}
}

// Tiles keep enough room for their logo, name and favorite control.
const (
	stationGap = 14
	tileHeight = 100
)

// radioHeader shows what the page lists, and how to narrow the list down.
func (a *app) radioHeader(c *ui.Context) {
	p := a.pal
	title, over, empty := "我的收藏", "你留住的声音", "还没有收藏的电台：在电台页点一张卡片右边的心，就留在这里。"
	switch a.page {
	case pageRadio:
		title, over, empty = "电台", "全球直播", ""
	case pageRecent:
		title, over, empty = "最近收听", "最近播过", "还没有听过的电台，听一个就会出现在这里。"
	}
	a.heading(c, title, over)
	if a.page != pageRadio {
		if len(a.pageStations()) == 0 {
			ui.Box(c).Height(56).Center().Children(func() {
				ui.Text(c, empty).FontSize(13).TextColor(p.muted).MaxLines(2)
			})
		}
		return
	}
	ui.Column(c).Gap(10).Margin(18, 0, 18, 0).Children(func() {
		a.browseState(c)
		if a.radio.name != "" {
			a.searchState(c)
		}
	})
	if len(a.radio.stations) > 0 || a.radio.moreErr != "" {
		status := ""
		switch {
		case a.radio.stale && a.radio.loading:
			status = "正在显示本地缓存，后台更新中…"
		case a.radio.err != "":
			status = a.radio.err
		}
		if status != "" {
			ui.Text(c, status).FontSize(12.5).TextColor(p.muted).Margin(4, 4, 12, 2)
		}
		return
	}
	ui.Box(c).Height(56).Center().Children(func() {
		switch {
		case a.radio.loading:
			ui.Text(c, "正在加载电台…").FontSize(13).TextColor(p.muted)
		case a.radio.err != "":
			ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.radio.err).FontSize(13).TextColor(p.muted)
				if pillButton(c.Key("retry"), nil, "重试", p.press.Over(p.bg), p.text).Clicked() {
					a.radio.asked = ""
					a.loadStations()
				}
			})
		default:
			ui.Text(c, "没有符合当前条件的电台，试试其他分类或重置筛选").FontSize(13).TextColor(p.muted)
		}
	})
}

// stationSearch is the compact title-bar field searching the directory by name.
func (a *app) stationSearch(c *ui.Context) {
	p := a.pal
	edge := p.hairline
	if a.stationBox.Focused(c) {
		edge = p.accent
	}
	ui.Row(c).Height(32).PaddingX(12).Gap(8).Radius(16).AlignItems(ui.Center).
		Background(p.surface).Border(1, edge).Children(func() {
		ui.Icon(c, iconSearch).Size(15, 15).TextColor(p.muted)
		in := ui.TextInputBase(c.Key("station-search"), &a.radio.query).Grow(1).MinWidth(0).Height(26).
			Placeholder("搜索电台，如 jazz、BBC…").FontSize(12.5).TextColor(p.text).FocusRing(false).Label("搜索电台")
		// Bound, so that Space types a space here rather than playing, and
		// so that Control-F brings the pointer to it.
		in.Bind(&a.stationBox)
		in.OnSubmit(a.searchStations)
	})
}

// searchStations applies the submitted name without losing active facets.
func (a *app) searchStations() {
	a.radio.name = strings.TrimSpace(a.radio.query)
	a.listScroll = ui.ListState{}
	a.loadStations()
}

// browseState combines directory facets; results always use popularity order.
func (a *app) browseState(c *ui.Context) {
	if a.radio.country == "" {
		a.radio.country = stationCountries[0].label
	}
	if a.radio.language == "" {
		a.radio.language = stationLanguages[0].label
	}
	changed := false
	ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Justify(ui.End).Children(func() {
		ui.Box(c.Key("content-filter")).Grow(1).Basis(500).MinWidth(0).Children(func() {
			labels := make([]string, len(stationTags))
			for i, tag := range stationTags {
				labels[i] = tag.label
			}
			if a.chips(c.Key("radio-tag"), &a.radio.tag, labels) {
				changed = true
			}
		})
		ui.Row(c.Key("location-filters")).Gap(12).Shrink(0).AlignItems(ui.Center).Children(func() {
			countries := make([]string, len(stationCountries))
			for i, country := range stationCountries {
				countries[i] = country.label
			}
			if ui.Select(c.Key("radio-country"), &a.radio.country, countries).
				Width(116).MinWidth(0).Height(30).Background(a.pal.surface).Radius(8).Label("地区").
				OnChange(a.saveRadioFilters).Changed() {
				changed = true
			}
			languages := make([]string, len(stationLanguages))
			for i, language := range stationLanguages {
				languages[i] = language.label
			}
			if ui.Select(c.Key("radio-language"), &a.radio.language, languages).
				Width(106).MinWidth(0).Height(30).Background(a.pal.surface).Radius(8).Label("语言").
				OnChange(a.saveRadioFilters).Changed() {
				changed = true
			}
		})
	})
	if changed {
		// The controls live inside the list header: request a scroll rather
		// than replacing the list state while its rows are being built.
		a.listScroll.ScrollTo(0, ui.Start)
		a.loadStations()
	}
}

// searchState clears the submitted name without clearing the other facets.
func (a *app) searchState(c *ui.Context) {
	p := a.pal
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "搜索").FontSize(12.5).FontWeight(600).TextColor(p.muted)
		q := a.radio.name
		if q == "" {
			q = "还没有输入"
		}
		ui.Text(c, "「"+q+"」").FontSize(12.5).TextColor(p.text).SingleLine()
		if a.radio.loading {
			ui.Text(c, "正在搜索…").FontSize(12).TextColor(p.faint)
		}
		back := ui.ButtonBase(c.Key("search-back")).Height(26).PaddingX(11).Radius(999).
			AlignItems(ui.Center).FocusRing(false).Label("清除搜索")
		bg := ui.Transparent
		if back.Hovered() {
			bg = p.hover
		}
		back.Background(back.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
		back.Children(func() { ui.Text(c, "清除搜索").FontSize(12).FontWeight(500).TextColor(p.accent) })
		if back.Clicked() {
			a.radio.query, a.radio.name = "", ""
			a.listScroll.ScrollTo(0, ui.Start)
			a.loadStations()
		}
	})
}

// chips is a row of buttons, one of which is chosen: how a page narrows
// down what it lists. It reports whether another was chosen.
func (a *app) chips(c *ui.Context, selected *int, labels []string) bool {
	p := a.pal
	before := *selected
	ui.Row(c).Gap(8).Wrap().Children(func() {
		for i, l := range labels {
			on := i == *selected
			b := ui.ButtonBase(c.Key(l)).Height(32).PaddingX(16).Radius(999).Border(1, p.hairline).AlignItems(ui.Center).FocusRing(false).Label(l)
			bg := p.surface
			switch {
			case on:
				bg = p.accent
				b.Border(1, p.accent).Shadow(0, 3, 10, -3, p.accent.Alpha(0.3))
			case b.Hovered():
				bg = p.hover
			}
			b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
			fg := p.text
			if on {
				fg = p.onAccent
			}
			b.Children(func() { ui.Text(c, l).FontSize(12.5).FontWeight(500).TextColor(fg).SingleLine() })
			if b.Clicked() && i != *selected {
				*selected = i
			}
		}
	})
	return *selected != before
}

// stationTile is a station of the grid: its logo beside its name, playing on
// a click, and kept by the heart at its corner.
func (a *app) stationTile(c *ui.Context, st radio.Station, i int) {
	p := a.pal
	playing := a.stationPlaying(st)
	playable := stationPlayable(st) || playing
	tile := ui.ButtonBase(c.Key(st.ID)).Height(tileHeight).Grow(1).Basis(0).MinWidth(0).
		Padding(14).Radius(12).Border(1, p.hairline).FocusRing(false).Label(st.Name)
	if !playable {
		tile.Tooltip("这个电台没有有效的 HTTP / HTTPS 播放地址")
	}
	hovered := tile.Hovered()
	bg := p.surface
	switch {
	case playing:
		bg = p.accent.Alpha(0.07).Over(p.surface)
		tile.Border(1, p.accent.Alpha(0.55))
	case hovered && playable:
		bg = p.accent.Alpha(0.035).Over(p.surface)
		tile.Border(1, p.accent.Alpha(0.35)).Shadow(0, 4, 14, -5, p.cardShade)
	}
	tile.Background(tile.AnimateColor("bg", bg, ui.Animation{Duration: 100 * time.Millisecond}))
	tile.Children(func() {
		ui.Row(c).Fill().Gap(12).AlignItems(ui.Center).Children(func() {
			a.stationBadge(c, st.Name, st.Icon, 50, 12, false)
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(5).Children(func() {
				name := p.text
				switch {
				case playing:
					name = p.accent
				case !playable:
					name = p.muted
				}
				ui.Text(c, st.Name).FontSize(13.5).FontWeight(600).TextColor(name).SingleLine()
				if meta := stationMeta(st); meta != "" {
					ui.Text(c, meta).FontSize(11).TextColor(p.accent).SingleLine()
				}
				line := stationLine(st)
				if !playable {
					line = "播放地址不可用"
				}
				ui.Text(c, line).FontSize(11.5).TextColor(p.muted).SingleLine()
			})
			ui.Column(c).Width(28).Gap(4).AlignItems(ui.Center).Children(func() {
				a.favoriteButton(c, st)
				if playing && a.pl.Playing() {
					a.nowPlayingBars(c)
				} else if playing {
					ui.Icon(c, iconPause).Size(10, 10).TextColor(p.accent)
				}
			})
		})
	})
	if tile.Clicked() && playable {
		a.playStation(i)
	}
}

// stationBadge shows a station's logo over a tile of the size and radius,
// with its initial underneath: a station without a logo, or one whose logo
// does not load, shows the initial alone rather than a radio that every
// other station shows too. onDark says the tile sits on a dark card.
func (a *app) stationBadge(c *ui.Context, name, icon string, size, radius float32, onDark bool) ui.Element {
	p := a.pal
	bg, ink := stationTint(name, p.dark)
	if onDark {
		bg, ink = white.Alpha(0.14), white.Alpha(0.92)
	}
	box := ui.Box(c).Size(size, size).Shrink(0).Radius(radius).Center().Background(bg)
	bmp := bitmap(a.art.getStationIcon(icon))
	in := artIn(box, bmp)
	box.Children(func() {
		if in < 1 {
			if initial := stationInitial(name); initial != "" {
				ui.Text(c, initial).FontSize(size * 0.4).FontWeight(700).
					TextColor(ink).Opacity(1 - in)
			} else {
				// Nothing to take an initial from: a radio, as for a page
				// with no station on it yet.
				ui.Icon(c, iconRadio).Size(size*0.38, size*0.38).TextColor(ink).Opacity(1 - in)
			}
		}
		artImage(c, bmp, radius, in)
	})
	return box
}

// stationTint is the color behind a station's initial, and the initial's
// own: pale enough for the letter to read on.
func stationTint(name string, dark bool) (bg, ink ui.Color) {
	h := stationHue(name)
	if dark {
		return ui.Oklch(0.34, 0.05, h), ui.Oklch(0.86, 0.06, h)
	}
	return ui.Oklch(0.93, 0.045, h), ui.Oklch(0.46, 0.1, h)
}

// stationHue is the hue a station's color takes, from its name: a station
// keeps its color, and the stations of a page rarely share one.
func stationHue(name string) float32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return float32(h.Sum32() % 360)
}

// stationInitial is the character a station shows when it has no logo: the
// first letter or digit of its name, which a Chinese name shows as its
// first character.
func stationInitial(name string) string {
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	return ""
}

// sortStations puts the stations the app decodes first, in the order they
// came in: the ones at the top of a page play.
func sortStations(list []radio.Station) {
	sort.SliceStable(list, func(i, j int) bool {
		return stationPlayable(list[i]) && !stationPlayable(list[j])
	})
}

// mpv probes codecs itself. Only reject missing or non-network addresses;
// never allow a station directory to make the app open local files.
func stationPlayable(st radio.Station) bool {
	u, err := url.Parse(st.Stream)
	return err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

// favoriteButton keeps a station, or lets it go. Its press stops here,
// rather than reaching the tile that plays the station.
func (a *app) favoriteButton(c *ui.Context, st radio.Station) {
	p := a.pal
	kept := a.lib.has(st.ID)
	b := ui.ButtonBase(c.Key("fav-"+st.ID)).Size(28, 28).Center().Radius(999).
		FocusRing(false).Label("收藏")
	b.Children(func() {
		icon, color := iconHeart, p.faint
		if kept {
			icon, color = iconHeartFill, p.accent
		} else if b.Hovered() {
			color = p.text
		}
		ui.Icon(c, icon).Size(15, 15).TextColor(color)
	})
	if b.Clicked() {
		a.lib.toggle(st)
	}
}
