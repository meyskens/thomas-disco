package music

import (
	"io"
	"log"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
)

func GlobalPlay(songSig chan PkgSong) {
	for {
		song := <-songSig
		go song.v.PlayQueue(song.data)
	}
}

type VoiceInstance struct {
	voice      *discordgo.VoiceConnection
	session    *discordgo.Session
	encoder    *dca.EncodeSession
	stream     *dca.StreamingSession
	queueMutex sync.Mutex
	audioMutex sync.Mutex
	nowPlaying Song
	queue      []Song
	recv       []int16
	guildID    string
	channelID  string
	speaking   bool
	pause      bool
	stop       bool
	skip       bool
	volume     int

	bitrate int
}

func NewVoiceInstance(birtate int) *VoiceInstance {
	return &VoiceInstance{
		volume:  265,
		bitrate: birtate,
	}
}

func (v *VoiceInstance) PlayQueue(song Song) {
	// add song to queue
	v.QueueAdd(song)
	if v.speaking {
		// the bot is playing
		return
	}
	go func() {
		v.audioMutex.Lock()
		defer v.audioMutex.Unlock()
		for {
			if len(v.queue) == 0 {
				return
			}
			v.nowPlaying = v.QueueGetSong()
			v.stop = false
			v.skip = false
			v.speaking = true
			v.pause = false
			v.voice.Speaking(true)

			v.DCA(v.nowPlaying.VideoURL)

			v.QueueRemoveFisrt()
			if v.stop {
				v.QueueRemove()
			}
			v.stop = false
			v.skip = false
			v.speaking = false
			v.voice.Speaking(false)
		}
	}()
}

// DCA
func (v *VoiceInstance) DCA(url string) {
	opts := dca.StdEncodeOptions
	opts.RawOutput = true
	opts.Bitrate = v.bitrate
	opts.Application = "lowdelay"
	opts.Volume = v.volume

	encodeSession, err := dca.EncodeFile(url, opts)
	if err != nil {
		log.Println("FATA: Failed creating an encoding session: ", err)
	}
	v.encoder = encodeSession
	done := make(chan error)
	stream := dca.NewStream(encodeSession, v.voice, done)
	v.stream = stream

	err = <-done
	if err != nil && err != io.EOF {
		log.Println("FATA: An error occured", err)
	}
	// Clean up incase something happened and ffmpeg is still running
	encodeSession.Cleanup()
}

// Stop stop the audio
func (v *VoiceInstance) Stop() {
	v.stop = true
	if v.encoder != nil {
		v.encoder.Cleanup()
	}
}

func (v *VoiceInstance) Skip() bool {
	if v.speaking {
		if v.pause {
			return true
		} else {
			if v.encoder != nil {
				v.encoder.Cleanup()
			}
		}
	}
	return false
}

// Pause pause the audio
func (v *VoiceInstance) Pause() {
	v.pause = true
	if v.stream != nil {
		v.stream.SetPaused(true)
	}
}

// Resume resume the audio
func (v *VoiceInstance) Resume() {
	v.pause = false
	if v.stream != nil {
		v.stream.SetPaused(false)
	}
}

func (v *VoiceInstance) SetVolume(vl int) {
	v.volume = int(float64(vl) / 100.0 * 256.0)
}
