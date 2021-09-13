package music

// SearchVoiceChannel search the voice channel id into from guild.
func (m *MusicCommand) SearchVoiceChannel(user string) (voiceChannelID string) {
	for _, g := range m.dg.State.Guilds {
		for _, v := range g.VoiceStates {
			if v.UserID == user {
				return v.ChannelID
			}
		}
	}
	return ""
}

// AddTimeDuration calculate the total time duration
func AddTimeDuration(t TimeDuration) (total TimeDuration) {
	total.Second = t.Second % 60
	t.Minute = t.Minute + t.Second/60
	total.Minute = t.Minute % 60
	t.Hour = t.Hour + t.Minute/60
	total.Hour = t.Hour % 24
	total.Day = t.Day + t.Hour/24
	return
}
