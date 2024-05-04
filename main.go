package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/log"
	"github.com/cheshir/ttlcache"
	"github.com/hugolgst/rich-go/client"
)

const statePlaying = "playing"

var (
	shortSleep    = 5 * time.Second
	longSleep     = time.Minute
	songCache     = ttlcache.New(time.Minute)
	artworkCache  = ttlcache.New(time.Minute)
	shareURLCache = ttlcache.New(time.Minute)
)

func main() {
	defer func() {
		_ = songCache.Close()
		_ = artworkCache.Close()
		_ = shareURLCache.Close()
	}()

	log.SetLevelFromString("warning")
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		log.SetLevelFromString(level)
	}
	ac := activityConnection{}
	defer func() { ac.stop() }()

	for {
		if !isRunning("MacOS/Music") {
			log.WithField("sleep", longSleep).Warn("Apple Music is not running")
			ac.stop()
			time.Sleep(longSleep)
			continue
		}
		if !(isRunning("MacOS/Discord") || isRunning("arrpc")) {
			log.WithField("sleep", longSleep).Warn("Discord is not running")
			ac.stop()
			time.Sleep(longSleep)
			continue
		}
		details, err := getNowPlaying()
		if err != nil {
			if strings.Contains(err.Error(), "(-1728)") {
				log.WithField("sleep", longSleep).Warn("Apple Music stopped running")
				ac.stop()
				time.Sleep(longSleep)
				continue
			}

			log.WithError(err).WithField("sleep", shortSleep).Error("will try again soon")
			ac.stop()
			time.Sleep(shortSleep)
			continue
		}

		if details.State != statePlaying {
			if ac.connected {
				log.Info("not playing")
				ac.stop()
			}
			time.Sleep(shortSleep)
			continue
		}

		if err := ac.play(details); err != nil {
			log.WithError(err).Warn("could not set activity, will retry later")
		}

		time.Sleep(shortSleep)
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func isRunning(app string) bool {
	bts, err := exec.Command("pgrep", "-f", app).CombinedOutput()
	return string(bts) != "" && err == nil
}

func tellMusic(s string) (string, error) {
	bts, err := exec.Command(
		"osascript",
		"-e", "tell application \"Music\"",
		"-e", s,
		"-e", "end tell",
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(bts)), err)
	}
	return strings.TrimSpace(string(bts)), nil
}

func getNowPlaying() (Details, error) {
	init := time.Now()
	defer func() {
		log.WithField("took", time.Since(init)).Info("got info")
	}()

	initialState, err := tellMusic("get {database id} of current track & {player position, player state}")
	if err != nil {
		return Details{}, err
	}

	songID, err := strconv.ParseInt(strings.Split(initialState, ", ")[0], 10, 64)
	if err != nil {
		return Details{}, err
	}

	position, err := strconv.ParseFloat(strings.Split(initialState, ", ")[1], 64)
	if err != nil {
		return Details{}, err
	}

	state := strings.Split(initialState, ", ")[2]
	if state != statePlaying {
		return Details{
			State: state,
		}, nil
	}

	cached, ok := songCache.Get(ttlcache.Int64Key(songID))
	if ok {
		log.WithField("songID", songID).Debug("got song from cache")
		return Details{
			Song:     cached.(Song),
			Position: position,
			State:    state,
		}, nil
	}

	name, err := tellMusic("get {name} of current track")
	if err != nil {
		return Details{}, err
	}
	artist, err := tellMusic("get {artist} of current track")
	if err != nil {
		return Details{}, err
	}
	album, err := tellMusic("get {album} of current track")
	if err != nil {
		return Details{}, err
	}
	yearDuration, err := tellMusic("get {year, duration} of current track")
	if err != nil {
		return Details{}, err
	}

	year, err := strconv.Atoi(strings.Split(yearDuration, ", ")[0])
	if err != nil {
		return Details{}, err
	}

	duration, err := strconv.ParseFloat(strings.Split(yearDuration, ", ")[1], 64)
	if err != nil {
		return Details{}, err
	}

	metadata, err := getMetadata(artist, album, name)
	if err != nil {
		return Details{}, err
	}

	song := Song{
		ID:       songID,
		Name:     name,
		Artist:   artist,
		Album:    album,
		Year:     year,
		Duration: duration,
		Artwork:  metadata.Artwork,
		ShareURL: metadata.ShareURL,
		ShareID:  metadata.ID,
	}

	songCache.Set(ttlcache.Int64Key(songID), song, 24*time.Hour)

	return Details{
		Song:     song,
		Position: position,
		State:    state,
	}, nil
}

type Details struct {
	Song     Song
	Position float64
	State    string
}

type Song struct {
	ID       int64
	Name     string
	Artist   string
	Album    string
	Year     int
	Duration float64
	Artwork  string
	ShareURL string
	ShareID  string
}

func getMetadata(artist, album, song string) (Metadata, error) {
	key := url.QueryEscape(strings.Join([]string{artist, album, song}, " "))
	artworkCached, artworkOk := artworkCache.Get(ttlcache.StringKey(key))
	shareURLCached, shareURLOk := shareURLCache.Get(ttlcache.StringKey(key))
	if artworkOk && shareURLOk {
		log.WithField("key", key).Debug("got album artwork from cache")
		return Metadata{
			Artwork:  artworkCached.(string),
			ShareURL: shareURLCached.(string),
		}, nil
	}

	baseURL := "https://tools.applemediaservices.com/api/apple-media/music/US/search.json?types=songs&limit=1"
	resp, err := http.Get(baseURL + "&term=" + key)
	if err != nil {
		return Metadata{}, err
	}
	defer resp.Body.Close()

	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		return Metadata{}, err
	}

	var result getMetadataResult
	if err := json.Unmarshal(bts, &result); err != nil {
		return Metadata{}, err
	}
	if len(result.Songs.Data) == 0 {
		return Metadata{}, nil
	}

	id := result.Songs.Data[0].ID
	artwork := result.Songs.Data[0].Attributes.Artwork.URL
	artwork = strings.Replace(artwork, "{w}", "512", 1)
	artwork = strings.Replace(artwork, "{h}", "512", 1)
	shareURL := result.Songs.Data[0].Attributes.URL

	artworkCache.Set(ttlcache.StringKey(key), artwork, time.Hour)
	shareURLCache.Set(ttlcache.StringKey(key), shareURL, time.Hour)
	return Metadata{
		ID:       id,
		Artwork:  artwork,
		ShareURL: shareURL,
	}, nil
}

type getMetadataResult struct {
	Songs struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				URL     string `json:"url"`
				Artwork struct {
					URL string `json:"url"`
				} `json:"artwork"`
			} `json:"attributes"`
		} `json:"data"`
	} `json:"songs"`
}

type Metadata struct {
	ID       string
	Artwork  string
	ShareURL string
}

type activityConnection struct {
	connected    bool
	lastSongID   int64
	lastPosition float64
}

func (ac *activityConnection) stop() {
	if ac.connected {
		client.Logout()
		ac.connected = false
		ac.lastPosition = 0.0
		ac.lastSongID = 0
	}
}

func (ac *activityConnection) play(details Details) error {
	song := details.Song
	if ac.lastSongID == song.ID {
		if details.Position >= ac.lastPosition {
			log.
				WithField("songID", song.ID).
				WithField("position", details.Position).
				Debug("ongoing activity, ignoring")
			return nil
		}
	}
	log.
		WithField("lastSongID", ac.lastSongID).
		WithField("songID", song.ID).
		WithField("lastPosition", ac.lastPosition).
		WithField("position", details.Position).
		Debug("new event")

	ac.lastPosition = details.Position
	ac.lastSongID = song.ID

	start := time.Now().Add(-1 * time.Duration(details.Position) * time.Second)
	// end := time.Now().Add(time.Duration(song.Duration-details.Position) * time.Second)
	if !ac.connected {
		if err := client.Login("861702238472241162"); err != nil {
			log.WithError(err).Fatal("could not create rich presence client")
		}
		ac.connected = true
	}

	var buttons []*client.Button
	if song.ShareURL != "" {
		buttons = append(buttons, &client.Button{
			Label: "Listen on Apple Music",
			Url:   song.ShareURL,
		})
	}
	if link := songlink(song); link != "" {
		buttons = append(buttons, &client.Button{
			Label: "View on SongLink",
			Url:   link,
		})
	}

	if err := client.SetActivity(client.Activity{
		State:      fmt.Sprintf("by %s (%s)", song.Artist, song.Album),
		Details:    song.Name,
		LargeImage: firstNonEmpty(song.Artwork, "applemusic"),
		SmallImage: "play",
		LargeText:  song.Name,
		SmallText:  fmt.Sprintf("%s by %s (%s)", song.Name, song.Artist, song.Album),
		Timestamps: &client.Timestamps{
			Start: timePtr(start),
			// End:   timePtr(end),
		},
		Buttons: buttons,
	}); err != nil {
		return err
	}

	log.WithField("song", song.Name).
		WithField("album", song.Album).
		WithField("artist", song.Artist).
		WithField("year", song.Year).
		WithField("duration", time.Duration(song.Duration)*time.Second).
		WithField("position", time.Duration(details.Position)*time.Second).
		WithField("songlink", songlink(song)).
		Warn("now playing")
	return nil
}

func songlink(song Song) string {
	if song.ShareID == "" {
		return ""
	}
	return fmt.Sprintf("https://song.link/i/%s", song.ShareID)
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
