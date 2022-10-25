# Discord's Rich Presence from Apple Music

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

---

It looks more or less like this:

<img width="334" alt="CleanShot 2022-10-25 at 00 44 31@2x" src="https://user-images.githubusercontent.com/245435/197677486-eebc2ecf-b8be-4de2-8eb7-650042718789.png">

---

###### Hat tip to:

- https://github.com/AB-Law/Apple-Music-Discord-Rich-Presence
- https://github.com/rohilpatel1/Apple-Music-Rich-Presence

And many other projects that do the same thing.
