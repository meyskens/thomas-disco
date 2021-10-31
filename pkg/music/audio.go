package music

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/meyskens/thomas-disco/pkg/dca"
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

	musicOpts MusicOptions

	bitrate int
}

func NewVoiceInstance(birtate int, opts MusicOptions) *VoiceInstance {
	return &VoiceInstance{
		volume:    265,
		bitrate:   birtate,
		musicOpts: opts,
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

			v.DCA(v.nowPlaying.VidID, v.nowPlaying.VideoURL, !v.nowPlaying.URLIsLocal)

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
func (v *VoiceInstance) DCA(name, url string, store bool) {
	if v.musicOpts.S3Bucket == "" {
		store = false
	}
	s3, err := NewS3(v.musicOpts.S3Endpoint, v.musicOpts.S3Region, v.musicOpts.S3Bucket, v.musicOpts.S3Access, v.musicOpts.S3Secret)
	if err != nil {
		log.Println("failed creating an S3 session: ", err)
	}

	opts := dca.StdEncodeOptions
	opts.RawOutput = true
	opts.Bitrate = v.bitrate
	opts.Application = "lowdelay"
	opts.Volume = v.volume

	var encodeSession *dca.EncodeSession
	var out *os.File

	s3File, err := s3.Get(name)
	if err == nil {
		log.Println("Got song from S3")
		store = false
		// found file on s3
		// download 100k bytes before encoding
		bufferedReader := bufio.NewReaderSize(s3File, 2*1024*1024)
		bufferedReader.Peek(100 * 1024)
		defer s3File.Close()

		encodeSession, err = dca.EncodeMem(bufferedReader, opts)
		if err != nil {
			log.Println("FATA: Failed creating an encoding session: ", err)
		}
	} else if store {
		// download the audio to s3
		resp, err := http.Get(url)
		if err != nil {
			log.Println("Error downloading audio:", err)
			return
		}
		defer resp.Body.Close()

		// store to disk using a teereader
		os.Remove(name) // if it exists is probably is corrupt!
		out, err = os.Create(name)
		if err != nil {
			log.Println("Error creating output file:", err)
			return
		}

		r := io.TeeReader(resp.Body, out)

		// download 100k bytes before encoding
		bufferedReader := bufio.NewReaderSize(r, 2*1024*1024)
		bufferedReader.Peek(100 * 1024)

		encodeSession, err = dca.EncodeMem(bufferedReader, opts)
		if err != nil {
			log.Println("FATA: Failed creating an encoding session: ", err)
		}
	} else {
		var err error
		encodeSession, err = dca.EncodeFile(url, opts)
		if err != nil {
			log.Println("FATA: Failed creating an encoding session: ", err)
		}
	}
	v.encoder = encodeSession
	done := make(chan error)
	stream := dca.NewStream(encodeSession, v.voice, done)
	defer encodeSession.Cleanup()
	v.stream = stream

	err = <-done
	if err != nil && err != io.EOF {
		log.Println("FATA: An error occured", err)
		return
	}
	if store && !encodeSession.Killed {
		go func() {
			out.Close()
			defer os.Remove(name)
			defer os.Remove(name + ".mp3")
			err := encodeToMP3(name)
			if err != nil {
				log.Println("failed encoding to mp3: ", err)
				return
			}
			f, err := os.Open(name + ".mp3")
			if err != nil {
				log.Println("failed opening file: ", err)
				return
			}
			defer f.Close()
			err = s3.Put(name, f)
			if err != nil {
				log.Println("failed uploading to s3: ", err)
				return
			}
			log.Printf("Uploaded %s to s3\n", name)
		}()
	}
}

// Stop stop the audio
func (v *VoiceInstance) Stop() {
	v.stop = true
	if v.encoder != nil {
		v.encoder.Kill()
	}
}

func (v *VoiceInstance) Skip() bool {
	if v.speaking {
		if v.pause {
			return true
		} else {
			if v.encoder != nil {
				v.encoder.Kill()
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

func encodeToMP3(file string) error {
	args := []string{
		"-stats",
		"-i", file,
		"-abr", "1",
		"-b:a", "320",
		file + ".mp3",
	}

	ffmpeg := exec.Command("ffmpeg", args...)

	// Starts the ffmpeg command
	err := ffmpeg.Start()
	if err != nil {
		return err
	}

	return ffmpeg.Wait()
}
