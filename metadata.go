package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/caarlos0/log"
	"github.com/jellydator/ttlcache/v3"
)

type SongMetadata struct {
	ID       string
	Artwork  string
	ShareURL string
}

const baseURL = "https://tools.applemediaservices.com/api/apple-media/music/US/search.json"

func get(url string, result interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bts, result); err != nil {
		return err
	}

	return nil
}

func getSongMetadata(key string) (SongMetadata, error) {
	var result getSongMetadataResult
	get(baseURL+"?types=songs&limit=1&term="+key, &result)

	if len(result.Songs.Data) == 0 {
		return SongMetadata{}, nil
	}

	id := result.Songs.Data[0].ID
	artwork := result.Songs.Data[0].Attributes.Artwork.URL
	artwork = strings.Replace(artwork, "{w}", "512", 1)
	artwork = strings.Replace(artwork, "{h}", "512", 1)

	return SongMetadata{
		ID:       id,
		Artwork:  artwork,
		ShareURL: result.Songs.Data[0].Attributes.URL,
	}, nil
}

func getArtistArtwork(key string) (string, error) {
	var result getArtistMetadataResult
	get(baseURL+"?types=artists&limit=1&term="+key, &result)

	if len(result.Artists.Data) == 0 {
		return "", nil
	}

	artwork := result.Artists.Data[0].Attributes.Artwork.URL
	artwork = strings.Replace(artwork, "{w}", "512", 1)
	artwork = strings.Replace(artwork, "{h}", "512", 1)

	return artwork, nil
}

func getMetadata(artist, album, song string) (Metadata, error) {
	key := url.QueryEscape(strings.Join([]string{artist, album, song}, " "))
	albumArtworkCached := cache.albumArtwork.Get(key)
	artistArtworkCached := cache.artistArtwork.Get(key)
	shareURLCached := cache.shareURL.Get(key)

	if albumArtworkCached != nil && shareURLCached != nil && artistArtworkCached != nil {
		log.WithField("key", key).Debug("got song info from cache")
		return Metadata{
			AlbumArtwork:  albumArtworkCached.Value(),
			ArtistArtwork: artistArtworkCached.Value(),
			ShareURL:      shareURLCached.Value(),
		}, nil
	}

	songMetadata, err := getSongMetadata(key)
	if err != nil {
		return Metadata{}, err
	}
	albumArtwork, err := getArtistArtwork(url.QueryEscape(artist))
	if err != nil {
		return Metadata{}, err
	}

	cache.albumArtwork.Set(key, songMetadata.Artwork, ttlcache.DefaultTTL)
	cache.shareURL.Set(key, songMetadata.ShareURL, ttlcache.DefaultTTL)
	cache.artistArtwork.Set(key, albumArtwork, ttlcache.DefaultTTL)
	return Metadata{
		ID:            songMetadata.ID,
		AlbumArtwork:  songMetadata.Artwork,
		ShareURL:      songMetadata.ShareURL,
		ArtistArtwork: albumArtwork,
	}, nil
}

type getSongMetadataResult struct {
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

type getArtistMetadataResult struct {
	Artists struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				URL     string `json:"url"`
				Artwork struct {
					URL string `json:"url"`
				} `json:"artwork"`
			} `json:"attributes"`
		} `json:"data"`
	} `json:"artists"`
}

type Metadata struct {
	ID            string
	AlbumArtwork  string
	ArtistArtwork string
	ShareURL      string
}
