package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/log"
	"github.com/hugolgst/rich-go/client"
)

func main() {
	if err := client.Login("861702238472241162"); err != nil {
		log.WithError(err).Fatal("could not create rich presence client")
	}

	for {
		if !running() {
			time.Sleep(time.Minute)
			continue
		}

		details, err := getSongDetails()
		if err != nil {
			log.WithError(err).Error("will try again soon")
			time.Sleep(10 * time.Second)
			continue
		}

		if strings.TrimSpace(details.State) != "playing" {
			log.Warn("not playing")
			time.Sleep(30 * time.Second)
			continue
		}

		song := details.Song
		start := time.Now().Add(-1 * time.Duration(details.Position) * time.Second)
		// end := time.Now().Add(time.Duration(song.Duration-details.Position) * time.Second)
		if err := client.SetActivity(client.Activity{
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
		}); err != nil {
			log.WithError(err).Error("could not set activity, will retry later")
			time.Sleep(10 * time.Second)
		}

		log.WithField("song", song.Name).
			WithField("album", song.Album).
			WithField("artist", song.Artist).
			WithField("state", strings.TrimSpace(details.State)).
			Info("reported")
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

func getSongDetails() (Details, error) {
	bts, err := exec.Command(
		"osascript",
		"-e", "tell application \"Music\"",
		"-e", "get {name, artist, album, year, duration} of current track & {player position, player state}",
		"-e", "end tell",
	).CombinedOutput()
	if err != nil {
		return Details{}, err
	}
	parts := strings.Split(string(bts), ", ")
	if len(parts) != 7 {
		return Details{}, fmt.Errorf("invalid output: %q", string(bts))
	}

	year, err := strconv.Atoi(parts[3])
	if err != nil {
		return Details{}, err
	}

	duration, err := strconv.ParseFloat(parts[4], 64)
	if err != nil {
		return Details{}, err
	}

	position, err := strconv.ParseFloat(parts[5], 64)
	if err != nil {
		return Details{}, err
	}

	url, err := getArtwork(parts[1], parts[2], parts[0])
	if err != nil {
		return Details{}, err
	}

	return Details{
		Song: Song{
			Name:     parts[0],
			Artist:   parts[1],
			Album:    parts[2],
			Year:     year,
			Duration: duration,
			Artwork:  url,
		},
		Position: position,
		State:    parts[6],
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

// TODO: cache this
func getArtwork(artist, album, song string) (string, error) {
	artist = strings.ReplaceAll(artist, " ", "+")
	album = strings.ReplaceAll(album+"+"+song, " ", "+")
	response, err := http.Get("https://itunes.apple.com/search?term=" + artist + "+" + album + "&limit=1&entity=song")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	var result getArtworkResult
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if result.ResultCount == 0 {
		return "", nil
	}
	return result.Results[0].ArtworkUrl100, nil
}

type getArtworkResult struct {
	ResultCount int `json:"resultCount"`
	Results     []struct {
		ArtworkUrl100 string `json:"artworkUrl100"`
	} `json:"results"`
}
