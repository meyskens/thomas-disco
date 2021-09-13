# Thomas Disco

Thomas Disco is a music bot for Discord. It is based on the princibles of [Thomas Bot](https://github.com/itfactory-tm/thomas-bot) thus the name.
The music logic is based on [MusicBot](https://github.com/ljgago/MusicBot) but reorganised and improved.  
Thomas Disco has a bit of personality, don't mind its disco moves and song references...

### Features:

- Search YouTube videos.
- Song queue.
- Support for skip, pause and resume.
- Spotify playlists.
- Slash commands!

#### Planned features:

- Queue management.
- Youtube link support.

### Build and install

You need to have installed in your system **go>1.16** and **ffmpeg>3.0**

### Use

Thomas Disco needs a Youtube API key and a Discord bot token.

```bash
disco serve --token <discord> --youtube-token <yt>
```

### Docker

The Dockerfile is built to require Docker's buildx to be built.
