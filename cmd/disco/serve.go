package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/meyskens/thomas-disco/pkg/music"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(NewServeCmd())
}

type serveCmdOptions struct {
	Token        string
	YouTubeToken string

	S3Access   string
	S3Bucket   string
	S3Secret   string
	S3Region   string
	S3Endpoint string

	dg *discordgo.Session
}

// NewServeCmd generates the `serve` command
func NewServeCmd() *cobra.Command {
	s := serveCmdOptions{}
	c := &cobra.Command{
		Use:     "serve",
		Short:   "Run the server",
		Long:    `This connects to Discord and handle all events and streams music`,
		RunE:    s.RunE,
		PreRunE: s.Validate,
	}

	c.Flags().StringVar(&s.Token, "token", "", "Discord Bot Token")
	c.Flags().StringVar(&s.YouTubeToken, "youtube-token", "", "YouTube API Token")
	c.Flags().StringVar(&s.S3Access, "s3-access", "", "S3 Access Key")
	c.Flags().StringVar(&s.S3Bucket, "s3-bucket", "", "S3 Bucket")
	c.Flags().StringVar(&s.S3Secret, "s3-secret", "", "S3 Secret Key")
	c.Flags().StringVar(&s.S3Region, "s3-region", "", "S3 Region")
	c.Flags().StringVar(&s.S3Endpoint, "s3-endpoint", "", "S3 Endpoint")

	c.MarkFlagRequired("token")
	c.MarkFlagRequired("youtube-token")

	return c
}

func (s *serveCmdOptions) Validate(cmd *cobra.Command, args []string) error {
	if s.Token == "" {
		return errors.New("No token specified")
	}

	return nil
}

func (s *serveCmdOptions) RunE(cmd *cobra.Command, args []string) error {
	log.Println("Thomas Disco!")
	log.Println("Is this real life or is this just staging?")
	var err error
	s.dg, err = discordgo.New("Bot " + s.Token)
	if err != nil {
		return err
	}

	err = s.dg.Open()
	if err != nil {
		return fmt.Errorf("error opening connection: %w", err)
	}
	defer s.dg.Close()

	mc, err := music.NewMusicCommand(s.dg, music.MusicOptions{
		YoutubeToken: s.YouTubeToken,
		S3Access:     s.S3Access,
		S3Bucket:     s.S3Bucket,
		S3Secret:     s.S3Secret,
		S3Region:     s.S3Region,
		S3Endpoint:   s.S3Endpoint,
	})
	if err != nil {
		return err
	}
	log.Println("Installing commands")
	err = mc.InstallSlashCommands(s.dg)
	if err != nil {
		return err
	}
	log.Println("Registering handlers")
	mc.Register()

	go func() {
		for {
			s.dg.UpdateListeningStatus("never gonna give music up")
			time.Sleep(time.Minute)
		}
	}()

	// get bot username
	user, err := s.dg.User("@me")
	if err != nil {
		return err
	}
	log.Printf("connected as %s#%s", user.Username, user.Discriminator)

	log.Println("Thomas Disco is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	return nil
}
