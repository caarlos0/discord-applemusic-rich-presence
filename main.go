package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/log"
	"github.com/cheshir/ttlcache"
	"github.com/hugolgst/rich-go/client"
)

const statePlaying = "playing"

func main() {
	ac := activityConnection{}
	for {
		if !isRunning() {
			ac.stop()
			time.Sleep(time.Minute)
			continue
		}

		details, err := getNowPlaying()
		if err != nil {
			log.WithError(err).Error("will try again soon")
			ac.stop()
			time.Sleep(5 * time.Second)
			continue
		}

		if details.State != statePlaying {
			if ac.connected {
				log.Info("not playing")
				ac.stop()
			}
			time.Sleep(5 * time.Second)
			continue
		}

		if err := ac.play(details); err != nil {
			log.WithError(err).Error("could not set activity, will retry later")
		}

		time.Sleep(5 * time.Second)
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func isRunning() bool {
	bts, err := exec.Command(
		"osascript",
		"-e", "tell application \"System Events\"",
		"-e", "count (every process whose name is \"Music\")",
		"-e", "end tell",
	).CombinedOutput()
	if err != nil {
		log.WithError(err).Fatal("could not check if Music is running")
	}
	return strings.TrimSpace(string(bts)) == "1" && err == nil
}

func tellMusic(s string) (string, error) {
	bts, err := exec.Command(
		"osascript",
		"-e", "tell application \"Music\"",
		"-e", s,
		"-e", "end tell",
	).CombinedOutput()
	return strings.TrimSpace(string(bts)), err
}

func getNowPlaying() (Details, error) {
	init := time.Now()
	defer func() {
		log.WithField("took", time.Since(init)).Info("got info")
	}()

	positionState, err := tellMusic("get {player position, player state}")
	if err != nil {
		return Details{}, err
	}

	state := strings.Split(positionState, ", ")[1]
	if state != statePlaying {
		return Details{
			State: state,
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

	position, err := strconv.ParseFloat(strings.Split(positionState, ", ")[0], 64)
	if err != nil {
		return Details{}, err
	}

	url, err := getArtwork(artist, album, name)
	if err != nil {
		return Details{}, err
	}

	return Details{
		Song: Song{
			Name:     name,
			Artist:   artist,
			Album:    album,
			Year:     year,
			Duration: duration,
			Artwork:  url,
		},
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
	Name     string
	Artist   string
	Album    string
	Year     int
	Duration float64
	Artwork  string
}

var artworkCache = ttlcache.New(time.Minute)

func getArtwork(artist, album, song string) (string, error) {
	artist = strings.ReplaceAll(artist, " ", "+")
	album = strings.ReplaceAll(album, " ", "+")
	song = strings.ReplaceAll(song, " ", "+")
	key := strings.Join([]string{artist, album, song}, "+")

	cached, ok := artworkCache.Get(ttlcache.StringKey(key))
	if ok {
		return cached.(string), nil
	}

	resp, err := http.Get("https://itunes.apple.com/search?term=" + key + "&limit=1&entity=song")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result getArtworkResult
	if err := json.Unmarshal(bts, &result); err != nil {
		return "", err
	}
	if result.ResultCount == 0 {
		return "", nil
	}
	url := result.Results[0].ArtworkUrl100
	artworkCache.Set(ttlcache.StringKey(key), url, time.Hour)
	return url, nil
}

type getArtworkResult struct {
	ResultCount int `json:"resultCount"`
	Results     []struct {
		ArtworkUrl100 string `json:"artworkUrl100"`
	} `json:"results"`
}

type activityConnection struct {
	connected bool
}

func (ac *activityConnection) stop() {
	if ac.connected {
		client.Logout()
		ac.connected = false
	}
}

func (ac *activityConnection) play(details Details) error {
	song := details.Song
	start := time.Now().Add(-1 * time.Duration(details.Position) * time.Second)
	// end := time.Now().Add(time.Duration(song.Duration-details.Position) * time.Second)
	searchURL := fmt.Sprintf("https://music.apple.com/us/search?term=%s", url.QueryEscape(song.Name+" "+song.Artist))
	if !ac.connected {
		if err := client.Login("861702238472241162"); err != nil {
			log.WithError(err).Fatal("could not create rich presence client")
		}
		ac.connected = true
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
		Buttons: []*client.Button{
			{
				Label: "Search on Apple Music",
				Url:   searchURL,
			},
		},
	}); err != nil {
		return err
	}

	log.WithField("song", details.Song.Name).
		WithField("album", details.Song.Album).
		WithField("artist", details.Song.Artist).
		WithField("year", details.Song.Year).
		WithField("duration", time.Duration(details.Song.Duration)*time.Second).
		WithField("position", time.Duration(details.Position)*time.Second).
		Info("now playing")
	return nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
