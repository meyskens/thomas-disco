package music

type TimeDuration struct {
	Day    int
	Hour   int
	Minute int
	Second int
}

type Song struct {
	ChannelID string
	User      string
	ID        string
	VidID     string
	Title     string
	Duration  string
	VideoURL  string
}

type PkgSong struct {
	data Song
	v    *VoiceInstance
}
