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

func main() {
	ac := activityConnection{}
	for {
		if !running() {
			ac.stop()
			time.Sleep(time.Minute)
			continue
		}

		details, err := getSongDetails()
		if err != nil {
			log.WithError(err).Error("will try again soon")
			ac.stop()
			time.Sleep(30 * time.Second)
			continue
		}

		if strings.TrimSpace(details.State) != "playing" {
			log.Info("not playing")
			ac.stop()
			time.Sleep(30 * time.Second)
			continue
		}
		err = ac.play(details)
		if err != nil {
			log.WithError(err).Error("could not set activity, will retry later")
		} else {
			log.WithField("song", details.Song.Name).
				WithField("album", details.Song.Album).
				WithField("artist", details.Song.Artist).
				WithField("state", strings.TrimSpace(details.State)).
				Info("reported")
		}

		time.Sleep(5 * time.Second)
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func running() bool {
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

func getPart(s string) (string, error) {
	bts, err := exec.Command(
		"osascript",
		"-e", "tell application \"Music\"",
		"-e", s,
		"-e", "end tell",
	).CombinedOutput()
	return strings.TrimSpace(string(bts)), err
}

func getSongDetails() (Details, error) {
	name, err := getPart("get {name} of current track")
	if err != nil {
		return Details{}, err
	}
	artist, err := getPart("get {artist} of current track")
	if err != nil {
		return Details{}, err
	}
	album, err := getPart("get {album} of current track")
	if err != nil {
		return Details{}, err
	}
	yearS, err := getPart("get {year} of current track")
	if err != nil {
		return Details{}, err
	}
	durationS, err := getPart("get {duration} of current track")
	if err != nil {
		return Details{}, err
	}
	positionS, err := getPart("get {player position}")
	if err != nil {
		return Details{}, err
	}
	state, err := getPart("get {player state}")
	if err != nil {
		return Details{}, err
	}

	year, err := strconv.Atoi(yearS)
	if err != nil {
		return Details{}, err
	}

	duration, err := strconv.ParseFloat(durationS, 64)
	if err != nil {
		return Details{}, err
	}

	position, err := strconv.ParseFloat(positionS, 64)
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

	return client.SetActivity(client.Activity{
		State:      "Listening",
		Details:    fmt.Sprintf("%s by %s (%s)", song.Name, song.Artist, song.Album),
		LargeImage: song.Artwork,
		SmallImage: "applemusic",
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
	})
}
