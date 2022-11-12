# Discord's Rich Presence from Apple Music

This is a simple binary that uses Apple Script to grab the current song being
played on Apple Music, and reports it as Discord Rich Presence.

You can leave it running "forever", and it should work in a loop.

## Install

To use it, simply install it with:

```sh
brew install caarlos0/tap/discord-applemusic-rich-presence
```

## Run

And then start and enable the service with:

```sh
brew services start caarlos0/tap/discord-applemusic-rich-presence
```

And that should do the trick 😃

## F.A.Q.

### How it looks like?

It looks more or less like this:

![Screenshot](https://user-images.githubusercontent.com/245435/201494021-4b75aa4b-fb59-4a36-9ee5-c2d6ebae627d.png)


### Can it look more like the Spotify integration?

No. Nothing I can do, AFAIK, it's a Discord limitation.

### Clicking in "Search in Apple Music" does not work...

Apparently... you can't click in buttons in your own Rich Presence.
Ask a friend to click on yours to see if it is really not working.

### Nothing happens...

Sometimes you'd need to restart the service and/or Discord.
No idea why, haven't catch a single error about it, it just stops working.

To restart:

```sh
brew services restart caarlos0/tap/discord-applemusic-rich-presence
```

### Where are the logs?

```sh
tail -f $(brew --prefix)/var/log/discord-applemusic-rich-presence.log
```

---

###### Hat tip to:

- https://github.com/AB-Law/Apple-Music-Discord-Rich-Presence
- https://github.com/rohilpatel1/Apple-Music-Rich-Presence

And many other projects that do the same thing.

