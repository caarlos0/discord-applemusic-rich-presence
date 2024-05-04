package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/caarlos0/log"
	"github.com/cheshir/ttlcache"
	"github.com/hugolgst/rich-go/client"
)

const statePlaying = "playing"

var (
	shortSleep = 5 * time.Second
	longSleep  = time.Minute
	cache      = Cache{}
)

type Cache struct {
	song          *ttlcache.Cache
	albumArtwork  *ttlcache.Cache
	artistArtwork *ttlcache.Cache
	shareURL      *ttlcache.Cache
}

func main() {
	cache = Cache{
		song:          ttlcache.New(time.Hour),
		albumArtwork:  ttlcache.New(time.Hour),
		artistArtwork: ttlcache.New(time.Hour),
		shareURL:      ttlcache.New(time.Hour),
	}
	defer func() {
		_ = cache.song.Close()
		_ = cache.albumArtwork.Close()
		_ = cache.artistArtwork.Close()
		_ = cache.shareURL.Close()
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

func isRunning(app string) bool {
	bts, err := exec.Command("pgrep", "-f", app).CombinedOutput()
	return string(bts) != "" && err == nil
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
		LargeImage: firstNonEmpty(song.AlbumArtwork, "applemusic"),
		SmallImage: firstNonEmpty(song.ArtistArtwork, "play"),
		LargeText:  song.Album,
		SmallText:  song.Artist,
		Timestamps: &client.Timestamps{
			Start: &start,
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
