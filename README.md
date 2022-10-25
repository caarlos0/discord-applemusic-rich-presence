# Discord Apple Music Rich Presence

This is a simple binary that uses Apple Script to grab the current song being
played on Apple Music, and reports it as Discord Rich Presence.

You can leave it running "forever", and it should work in a loop.

To use it, simply install it with:

```sh
brew install caarlos0/tap/discord-applemusic-rich-presence
```

And then start and enable the service with:

```sh
brew services start caarlos0/tap/discord-applemusic-rich-presence
```

And that should do the trick 😃
