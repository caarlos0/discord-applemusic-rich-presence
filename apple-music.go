package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/log"
	"github.com/jellydator/ttlcache/v3"
)

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

	cached := cache.song.Get(songID)
	if cached != nil {
		log.WithField("songID", songID).Debug("got song from cache")
		return Details{
			Song:     cached.Value(),
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
		ID:            songID,
		Name:          name,
		Artist:        artist,
		Album:         album,
		Year:          year,
		Duration:      duration,
		AlbumArtwork:  metadata.AlbumArtwork,
		ArtistArtwork: metadata.ArtistArtwork,
		ShareURL:      metadata.ShareURL,
		ShareID:       metadata.ID,
	}

	cache.song.Set(songID, song, ttlcache.DefaultTTL)

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
	ID            int64
	Name          string
	Artist        string
	Album         string
	Year          int
	Duration      float64
	AlbumArtwork  string
	ArtistArtwork string
	ShareURL      string
	ShareID       string
}
