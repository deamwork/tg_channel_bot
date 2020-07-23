package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/asdine/storm"
	f "github.com/deamwork/tg_channel_bot/fetchers"
	tb "github.com/ihciah/telebot"
	bolt "go.etcd.io/bbolt"
)

const MaxAlbumSize = 10

type TelegramBot struct {
	Bot            *tb.Bot
	Database       *storm.DB
	Token          string        `json:"token"`
	Timeout        int           `json:"timeout"`
	DatabasePath   string        `json:"database"`
	FetcherConfigs FetcherConfig `json:"fetcher_config"`
	Channels       *[]*Channel
	Admins         []string `json:"admins"`
}

func (t *TelegramBot) LoadConfigFromEnv() {
	t.Token = os.Getenv("BOT_TOKEN")
	t.DatabasePath = "database.db"
	t.Timeout = 120

	admin := []string{os.Getenv("ADMIN_NAME")}
	t.Admins = admin

	var tumblrFetcher f.TumblrFetcher
	tumblrFetcher.OAuthConsumerKey = os.Getenv("TUMBLR_KEY")
	tumblrFetcher.OAuthConsumerSecret = os.Getenv("TUMBLR_SECRET")
	tumblrFetcher.OAuthToken = os.Getenv("TUMBLR_TOKEN")
	tumblrFetcher.OAuthTokenSecret = os.Getenv("TUMBLR_TOKEN_SECRET")

	var twitterFetcher f.TwitterFetcher
	twitterFetcher.AccessTokenSecret = os.Getenv("TWITTER_TOKEN_SECRET")
	twitterFetcher.AccessToken = os.Getenv("TWITTER_TOKEN")
	twitterFetcher.ConsumerKey = os.Getenv("TWITTER_KEY")
	twitterFetcher.ConsumerSecret = os.Getenv("TWITTER_SECRET")

	var fetcherConfig FetcherConfig
	fetcherConfig.Tumblr = tumblrFetcher
	fetcherConfig.Twitter = twitterFetcher

	t.FetcherConfigs = fetcherConfig

	var err error
	t.Bot, err = tb.NewBot(tb.Settings{
		Token:       t.Token,
		Poller:      &tb.LongPoller{Timeout: time.Duration(t.Timeout) * time.Second},
		HTTPTimeout: t.Timeout,
	})
	if err != nil {
		log.Fatal("[Cannot initialize telegram Bot]", err)
		return
	}

	t.Database, err = storm.Open(t.DatabasePath, storm.BoltOptions(0600, &bolt.Options{Timeout: 5 * time.Second}))
	if err != nil {
		log.Fatal("[Cannot initialize database]", err)
	}
	log.Printf("[Bot initialized]Token: %s\nTimeout: %d\n", t.Token, t.Timeout)
}

func (t *TelegramBot) LoadConfig(json_path string) {
	data, err := ioutil.ReadFile(json_path)
	if err != nil {
		log.Fatal("[Cannot read telegram config]", err)
		return
	}
	if err := json.Unmarshal(data, t); err != nil {
		log.Fatal("[Cannot parse telegram config]", err)
		return
	}
	t.Bot, err = tb.NewBot(tb.Settings{
		Token:       t.Token,
		Poller:      &tb.LongPoller{Timeout: time.Duration(t.Timeout) * time.Second},
		HTTPTimeout: t.Timeout,
	})
	if err != nil {
		log.Fatal("[Cannot initialize telegram Bot]", err)
		return
	}

	t.Database, err = storm.Open(t.DatabasePath, storm.BoltOptions(0600, &bolt.Options{Timeout: 5 * time.Second}))
	if err != nil {
		log.Fatal("[Cannot initialize database]", err)
	}
	log.Printf("[Bot initialized]Token: %s\nTimeout: %d\n", t.Token, t.Timeout)
}

func (t *TelegramBot) Serve() {
	t.RegisterHandler()
	t.Bot.Start()
}

func (t *TelegramBot) Send(to tb.Recipient, message f.ReplyMessage) error {
	if message.Err != nil {
		return message.Err
	}

	if len(message.Resources) == 1 {
		var err error
		var mediaFile tb.InputMedia
		if message.Resources[0].T == f.TIMAGE {
			mediaFile = &tb.Photo{File: tb.FromURL(message.Resources[0].URL), Caption: message.Resources[0].Caption}
		} else if message.Resources[0].T == f.TVIDEO {
			mediaFile = &tb.Video{File: tb.FromURL(message.Resources[0].URL), Caption: message.Resources[0].Caption}
		} else {
			return errors.New("Undefined message type.")
		}
		_, err = t.Bot.Send(to, mediaFile)
		return err
	}

	if len(message.Resources) == 0 {
		if _, err := t.Bot.Send(to, message.Caption); err != nil {
			log.Println("Unable to send text:", message.Caption)
			return err
		} else {
			log.Println("Sent text:", message.Caption)
		}
	}

	var ret error
	for i := 0; i < len(message.Resources); i += MaxAlbumSize {
		end := i + MaxAlbumSize
		if end > len(message.Resources) {
			end = len(message.Resources)
		}
		mediaFiles := make(tb.Album, 0, MaxAlbumSize)
		for _, r := range message.Resources[i:end] {
			if r.T == f.TIMAGE {
				mediaFiles = append(mediaFiles, &tb.Photo{File: tb.FromURL(r.URL), Caption: r.Caption})
			} else if r.T == f.TVIDEO {
				mediaFiles = append(mediaFiles, &tb.Video{File: tb.FromURL(r.URL), Caption: r.Caption})
			} else {
				continue
			}
		}
		if _, err := t.Bot.SendAlbum(to, mediaFiles); err != nil {
			log.Println("Unable to send album", err)
			ret = err
		} else {
			log.Println("Sent album")
		}
	}
	return ret
}

func (t *TelegramBot) SendAll(to tb.Recipient, messages []f.ReplyMessage) (err error) {
	err = nil
	for _, msg := range messages {
		go t.Send(to, msg)
	}
	return
}
