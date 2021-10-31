package music

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	ytdl "github.com/kkdai/youtube/v2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func getDuration(stringRawFull, stringRawOffset string) (stringRemain string) {
	var stringFull string
	var duration TimeDuration
	var partial time.Duration

	stringFull = strings.Replace(stringRawFull, "P", "", 1)
	stringFull = strings.Replace(stringFull, "T", "", 1)
	stringFull = strings.ToLower(stringFull)

	var secondsFull, secondsOffset int
	value := strings.Split(stringFull, "d")
	if len(value) == 2 {
		secondsFull, _ = strconv.Atoi(value[0])
		// get the days in seconds
		secondsFull = secondsFull * 86400
		// get the format 1h1m1s in seconds
		partial, _ = time.ParseDuration(value[1])
		secondsFull = secondsFull + int(partial.Seconds())
	} else {
		partial, _ = time.ParseDuration(stringFull)
		secondsFull = int(partial.Seconds())
	}

	if stringRawOffset != "" {
		value = strings.Split(stringRawOffset, "s")
		if len(value) == 2 {
			secondsOffset, _ = strconv.Atoi(value[0])
		}
	}
	// substact the time offset
	duration.Second = secondsFull - secondsOffset

	if duration.Second <= 0 {
		return "0:00"
	}

	// print the time
	t := AddTimeDuration(duration)
	if t.Day == 0 && t.Hour == 0 {
		return fmt.Sprintf("%02d:%02d", t.Minute, t.Second)
	}
	if t.Day == 0 {
		return fmt.Sprintf("%02d:%02d:%02d", t.Hour, t.Minute, t.Second)
	}
	return fmt.Sprintf("%d:%02d:%02d:%02d", t.Day, t.Hour, t.Minute, t.Second)
}

func (m *MusicCommand) YoutubeFind(searchString, uID, chID string, v *VoiceInstance) (song_struct PkgSong, err error) { //(url, title, time string, err error)

	var timeOffset string
	if strings.Contains(searchString, "?t=") || strings.Contains(searchString, "&t=") {
		var split []string
		switch {
		case strings.Contains(searchString, "?t="):
			split = strings.Split(searchString, "?t=")
		case strings.Contains(searchString, "&t="):
			split = strings.Split(searchString, "&t=")
		}
		searchString = split[0]
		timeOffset = split[1]

		if !strings.ContainsAny(timeOffset, "h | m | s") {
			timeOffset = timeOffset + "s" // secons
		}
	}

	var audioId, audioTitle, duration string
	audioId, audioTitle, duration, err = m.OfficialSearch(searchString)
	if err != nil {
		log.Println("Using unofficial search")
		audioId, audioTitle, duration, err = m.UnofficialSearch(searchString)
		if err != nil {
			return
		}
	}

	if audioId == "" {
		err = errors.New("no song found")
		return
	}

	yt := ytdl.Client{}

	vid, err := yt.GetVideo("https://www.youtube.com/watch?v=" + audioId)
	if err != nil {
		err = fmt.Errorf("error getting video: %w", err)
		return
	}

	formats := vid.Formats.WithAudioChannels()
	formats.Sort()
	if len(formats) == 0 {
		err = fmt.Errorf("no audio format found")
		return
	}

	bestFormat := formats[0]
	for _, format := range formats {
		if format.AudioQuality == bestFormat.AudioQuality {
			break
		}
		if format.AudioQuality == "AUDIO_QUALITY_MEDIUM" && bestFormat.AudioQuality == "AUDIO_QUALITY_LOW" {
			bestFormat = format
		}
		if format.AudioQuality == "AUDIO_QUALITY_HIGH" && (bestFormat.AudioQuality == "AUDIO_QUALITY_LOW" || bestFormat.AudioQuality == "AUDIO_QUALITY_MEDIUM") {
			bestFormat = format
		}
	}

	videoURLString, err := yt.GetStreamURL(vid, &bestFormat)
	if err != nil {
		err = fmt.Errorf("error getting video URL: %v", err)
		return
	}

	durationString := getDuration(duration, timeOffset)

	videoURL, _ := url.Parse(videoURLString)
	if videoURL != nil {
		if timeOffset != "" {

			offset, _ := time.ParseDuration(timeOffset)
			query := videoURL.Query()
			query.Set("begin", fmt.Sprint(int64(offset/time.Millisecond)))
			videoURL.RawQuery = query.Encode()
		}
		videoURLString = videoURL.String()
	} else {
		log.Println("Video URL not found")
	}

	song := Song{
		chID,
		uID,
		uID,
		vid.ID,
		audioTitle,
		durationString,
		videoURLString,
		false,
	}

	song_struct.data = song
	song_struct.v = v

	return
}

type YTReply struct {
	Results []struct {
		Channel struct {
			ID              string `json:"id"`
			Title           string `json:"title"`
			URL             string `json:"url"`
			Snippet         string `json:"snippet"`
			ThumbnailSrc    string `json:"thumbnail_src"`
			VideoCount      string `json:"video_count"`
			SubscriberCount string `json:"subscriber_count"`
			Verified        bool   `json:"verified"`
		} `json:"channel,omitempty"`
		Video struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			URL          string `json:"url"`
			Duration     string `json:"duration"`
			Snippet      string `json:"snippet"`
			UploadDate   string `json:"upload_date"`
			ThumbnailSrc string `json:"thumbnail_src"`
			Views        string `json:"views"`
		} `json:"video,omitempty"`
		Uploader struct {
			Username string `json:"username"`
			URL      string `json:"url"`
			Verified bool   `json:"verified"`
		} `json:"uploader,omitempty"`
		Playlist struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			URL          string `json:"url"`
			ThumbnailSrc string `json:"thumbnail_src"`
			VideoCount   string `json:"video_count"`
		} `json:"playlist,omitempty"`
	} `json:"results"`
	Version          string `json:"version"`
	Parser           string `json:"parser"`
	Key              string `json:"key"`
	EstimatedResults string `json:"estimatedResults"`
	NextPageToken    string `json:"nextPageToken"`
}

func (m *MusicCommand) UnofficialSearch(query string) (string, string, string, error) {
	resp, err := http.Get(fmt.Sprintf("https://youtube-scrape.herokuapp.com/api/search?q=%s", url.QueryEscape(query)))
	if err != nil {
		return "", "", "", err
	}

	defer resp.Body.Close()
	data := &YTReply{}
	err = json.NewDecoder(resp.Body).Decode(data)
	if err != nil {
		return "", "", "", err
	}

	if len(data.Results) == 0 {
		return "", "", "", errors.New("no results found")
	}

	return data.Results[0].Video.ID, data.Results[0].Video.Title, data.Results[0].Video.Duration, nil
}

func (m *MusicCommand) OfficialSearch(query string) (string, string, string, error) {
	service, err := youtube.NewService(context.TODO(), option.WithAPIKey(m.opts.YoutubeToken))
	if err != nil {
		return "", "", "", err
	}

	call := service.Search.List([]string{"id", "snippet"}).Q(query).MaxResults(1)
	response, err := call.Do()
	if err != nil {
		return "", "", "", err
	}

	if len(response.Items) == 0 {
		return "", "", "", errors.New("no results found")
	}

	id := response.Items[0].Id.VideoId
	title := response.Items[0].Snippet.Title

	videos := service.Videos.List([]string{"contentDetails"}).Id(id)
	resp, err := videos.Do()
	if err != nil {
		err = fmt.Errorf("error getting video details: %v", err)
		return "", "", "", err
	}

	if len(resp.Items) == 0 {
		err = fmt.Errorf("no video details found")
		return "", "", "", err
	}

	return id, title, resp.Items[0].ContentDetails.Duration, nil
}
