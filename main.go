package main

import (
	"fmt"
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

		song := details.Song
		state := "Paused"
		if strings.TrimSpace(details.State) == "playing" {
			state = fmt.Sprintf("Playing on %s...", song.Album)
		}

		if err := client.SetActivity(client.Activity{
			State:      state,
			Details:    fmt.Sprintf("%s - %s", song.Artist, song.Name),
			LargeImage: "applemusic",
			SmallImage: "play",
			LargeText:  state,
			SmallText:  fmt.Sprintf("Listening to %s - %s (%s)", song.Artist, song.Name, song.Album),
			Timestamps: &client.Timestamps{
				Start: timePtr(time.Now().Add(-1 * time.Duration(details.Position) * time.Second)),
				// End: ,
			},
		}); err != nil {
			log.WithError(err).Fatal("could not create rich presence client")
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

	return Details{
		Song: Song{
			Name:     parts[0],
			Artist:   parts[1],
			Album:    parts[2],
			Year:     year,
			Duration: duration,
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
}
