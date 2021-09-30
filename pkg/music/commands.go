package music

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/itfactory-tm/thomas-bot/pkg/util/slash"
)

type MusicCommand struct {
	dg *discordgo.Session

	voiceInstances map[string]*VoiceInstance
	mutex          sync.Mutex
	songSignal     chan PkgSong
	YoutubeToken   string

	SpotifyTokenMutex sync.Mutex
	SpotifyToken      string
}

func NewMusicCommand(dg *discordgo.Session, yt string) (*MusicCommand, error) {
	songSignal := make(chan PkgSong)
	go GlobalPlay(songSignal)

	return &MusicCommand{
		dg:             dg,
		voiceInstances: map[string]*VoiceInstance{},
		songSignal:     songSignal,
		YoutubeToken:   yt,
	}, nil
}

func (m *MusicCommand) Register() {
	m.dg.AddHandler(func(sess *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			if i.ApplicationCommandData().Name == "join" {
				m.Join(i, false)
			} else if i.ApplicationCommandData().Name == "play" {
				m.Play(i)
			} else if i.ApplicationCommandData().Name == "disconnect" {
				m.Leave(i)
			} else if i.ApplicationCommandData().Name == "skip" {
				m.Skip(i)
			} else if i.ApplicationCommandData().Name == "pause" || i.ApplicationCommandData().Name == "stop" {
				m.Pause(i)
			} else if i.ApplicationCommandData().Name == "resume" {
				m.Resume(i)
			} else if i.ApplicationCommandData().Name == "volume" {
				m.Volume(i)
			} else if i.ApplicationCommandData().Name == "playlist" {
				m.Playlist(i)
			}
		}
	})
}

// InstallSlashCommands registers the slash commands
func (m *MusicCommand) InstallSlashCommands(session *discordgo.Session) error {
	err := slash.InstallSlashCommand(session, "", discordgo.ApplicationCommand{
		Name:        "join",
		Description: "Add the bot to a VC",
		Options:     []*discordgo.ApplicationCommandOption{},
	})
	if err != nil {
		return err
	}

	err = slash.InstallSlashCommand(session, "", discordgo.ApplicationCommand{
		Name:        "disconnect",
		Description: "Remove the bot from a VC",
		Options:     []*discordgo.ApplicationCommandOption{},
	})
	if err != nil {
		return err
	}

	err = slash.InstallSlashCommand(session, "", discordgo.ApplicationCommand{
		Name:        "play",
		Description: "Play a song",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "query",
				Description: "query for song, can be a youtube URL or text query",
				Required:    true,
			},
		},
	})
	if err != nil {
		return err
	}

	err = slash.InstallSlashCommand(session, "", discordgo.ApplicationCommand{
		Name:        "skip",
		Description: "Skip to the next song",
		Options:     []*discordgo.ApplicationCommandOption{},
	})
	if err != nil {
		return err
	}

	err = slash.InstallSlashCommand(session, "", discordgo.ApplicationCommand{
		Name:        "stop",
		Description: "Stop playing song",
		Options:     []*discordgo.ApplicationCommandOption{},
	})
	if err != nil {
		return err
	}

	err = slash.InstallSlashCommand(session, "", discordgo.ApplicationCommand{
		Name:        "pause",
		Description: "Pause current song",
		Options:     []*discordgo.ApplicationCommandOption{},
	})
	if err != nil {
		return err
	}

	err = slash.InstallSlashCommand(session, "", discordgo.ApplicationCommand{
		Name:        "resume",
		Description: "Resume playing song",
		Options:     []*discordgo.ApplicationCommandOption{},
	})
	if err != nil {
		return err
	}

	err = slash.InstallSlashCommand(session, "", discordgo.ApplicationCommand{
		Name:        "volume",
		Description: "Set audio volume",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "volume",
				Description: "Volume from 0 to 100",
				Required:    true,
			},
		},
	})
	if err != nil {
		return err
	}

	err = slash.InstallSlashCommand(session, "", discordgo.ApplicationCommand{
		Name:        "playlist",
		Description: "Play a spotify playlist",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "link",
				Description: "Link to a spotify playlist",
				Required:    true,
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (mc *MusicCommand) Join(i *discordgo.InteractionCreate, doNotReply bool) error {
	v := mc.voiceInstances[i.GuildID]
	voiceChannelID := mc.SearchVoiceChannel(i.Member.User.ID)
	if voiceChannelID == "" {
		mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are not in voice!",
				Flags:   64, // hidden
			},
		})
		return errors.New("You are not in voice!")
	}
	if v != nil {
		log.Println("INFO: Voice Instance already created.")
	} else {
		vc, err := mc.dg.Channel(voiceChannelID)
		if err != nil {
			mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "I have troubles joining you, I'm sorry :(",
					Flags:   64, // hidden
				},
			})
			return errors.New("I have troubles joining you, I'm sorry :(")
		}
		// create new voice instance
		mc.mutex.Lock()
		v = NewVoiceInstance(vc.Bitrate / 1000)
		mc.voiceInstances[i.GuildID] = v
		v.guildID = i.GuildID
		v.session = mc.dg
		mc.mutex.Unlock()
	}
	var err error
	v.voice, err = mc.dg.ChannelVoiceJoin(v.guildID, voiceChannelID, false, false)
	if err != nil {
		v.Stop()
		time.Sleep(200 * time.Millisecond)
		v.voice.Disconnect()
		mc.mutex.Lock()
		delete(mc.voiceInstances, v.guildID)
		mc.mutex.Unlock()

		mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "I have troubles joining you, I'm sorry :(",
				Flags:   64, // hidden
			},
		})
		return errors.New("I have troubles joining you, I'm sorry :(")
	}
	v.voice.Speaking(false)
	if !doNotReply {
		mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "I joined your VC, let's boogie!",
			},
		})
	}
	return nil
}

func (mc *MusicCommand) Leave(i *discordgo.InteractionCreate) {
	v := mc.CheckVC(i, true)
	if v == nil {
		return
	}
	v.Stop()
	time.Sleep(200 * time.Millisecond)
	v.voice.Disconnect()
	log.Println("INFO: Voice channel destroyed")
	mc.mutex.Lock()
	delete(mc.voiceInstances, v.guildID)
	mc.mutex.Unlock()
	mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "BYE BYE BYE",
		},
	})
}

func (mc *MusicCommand) Play(i *discordgo.InteractionCreate) {
	mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	v := mc.CheckVC(i, false)
	if v == nil {
		err := mc.Join(i, true)
		if err != nil {
			mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
				Content: err.Error(),
			})
		}
		v = mc.CheckVC(i, true)
		if v == nil { // try twice
			return
		}
	}

	query := i.ApplicationCommandData().Options[0].Value.(string)

	if len(query) < 1 {
		mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
			Content: "You need to give me some content to look for!",
		})
		return
	}
	// if the user is not a voice channel not accept the command
	voiceChannelID := mc.SearchVoiceChannel(i.Member.User.ID)
	if v.voice.ChannelID != voiceChannelID {
		mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
			Content: "Do I know you? I was not in your VC! You need to do /join first",
		})
		return
	}
	// send play my_song_youtube
	song, err := mc.YoutubeFind(query, i.Member.User.ID, i.ChannelID, v)
	if err != nil || song.data.ID == "" {
		log.Println("ERROR: Youtube search: ", err)
		mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
			Content: "I do not know how to groove to that song...",
		})

		return
	}

	mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
		Content: fmt.Sprintf("Let's dance to %q", song.data.Title),
	})

	go func() {
		mc.songSignal <- song
	}()
}

func (mc *MusicCommand) Skip(i *discordgo.InteractionCreate) {
	v := mc.CheckVC(i, true)
	if v == nil {
		return
	}
	if len(v.queue) == 0 {
		mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "No songs are playing, I cannot skip skip skips a beat",
				Flags:   64, // hidden
			},
		})
		return
	}
	v.Skip()

	mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Don't like this one? Skipped!",
		},
	})
}

func (mc *MusicCommand) Pause(i *discordgo.InteractionCreate) {
	v := mc.CheckVC(i, true)
	if v == nil {
		return
	}

	v.Pause()

	mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Stopped (in the name of love!)",
		},
	})
}

func (mc *MusicCommand) Resume(i *discordgo.InteractionCreate) {
	v := mc.CheckVC(i, true)
	if v == nil {
		return
	}

	v.Resume()

	mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Let's get this party going!",
		},
	})
}

func (mc *MusicCommand) Volume(i *discordgo.InteractionCreate) {
	v := mc.CheckVC(i, true)
	if v == nil {
		return
	}

	volume := int(i.ApplicationCommandData().Options[0].Value.(float64))
	if volume < 0 || volume > 111 { // > 100 is not advertised but it allows for a joke
		mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "I can't change the volume to anything not between 0 and 100",
				Flags:   64, // hidden
			},
		})
		return
	}
	v.SetVolume(volume)

	mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Turn it up to 11, I mean %d this will apply to the next song", volume),
		},
	})
}

func (mc *MusicCommand) Playlist(i *discordgo.InteractionCreate) {
	mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	v := mc.CheckVC(i, false)
	if v == nil {
		err := mc.Join(i, true)
		if err != nil {
			mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
				Content: err.Error(),
			})
		}
		v = mc.CheckVC(i, true)
		if v == nil { // try twice
			return
		}
	}

	// if the user is not a voice channel not accept the command
	voiceChannelID := mc.SearchVoiceChannel(i.Member.User.ID)
	if v.voice.ChannelID != voiceChannelID {
		mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
			Content: "Do I know you? I was not in your VC! You need to do /join first",
		})
		return
	}

	link := i.ApplicationCommandData().Options[0].Value.(string)

	if len(link) < 1 {
		mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
			Content: "You need to give me some content to look for!",
		})
		return
	}

	url, err := url.Parse(link)
	if err != nil || url.Hostname() != "open.spotify.com" {
		mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
			Content: "Woah this is a weird link... I only know ones that start with open.spotify.com",
		})
		return
	}

	id := strings.Split(url.Path, "/")[len(strings.Split(url.Path, "/"))-1]
	log.Printf("Spotify ID %s", id)

	tracks := mc.SpotifyToSearch(id)
	if len(tracks) == 0 {
		mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
			Content: "I couldn't load the playlist",
		})
		return
	}
	foundTracks := []string{}

	for _, track := range tracks {
		log.Printf("Looking for %s", track)
		// send play my_song_youtube
		song, err := mc.YoutubeFind(track, i.Member.User.ID, i.ChannelID, v)
		if err != nil || song.data.ID == "" {
			log.Println(err)
			continue
		}
		foundTracks = append(foundTracks, song.data.Title)
		go func() {
			mc.songSignal <- song
		}()

		time.Sleep(200 * time.Millisecond)
	}

	if len(foundTracks) == 0 {
		mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
			Content: "I couldn't load the songs",
		})
		return
	}

	if len(foundTracks) > 20 {
		foundTracks = foundTracks[:20]
	}

	mc.dg.InteractionResponseEdit(mc.dg.State.User.ID, i.Interaction, &discordgo.WebhookEdit{
		Content: fmt.Sprintf("Loaded that mixtape, buddy! Let's dance songs like:\n%s", strings.Join(foundTracks, "\n")),
	})
}

func (mc *MusicCommand) CheckVC(i *discordgo.InteractionCreate, reply bool) *VoiceInstance {
	v := mc.voiceInstances[i.GuildID]
	if v == nil && reply {
		mc.dg.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Do I know you? I was not in your VC! You need to do /join first",
				Flags:   64, // hidden
			},
		})
		return nil
	}

	return v
}
