package main

import (
	"strconv"
	"strings"

	f "github.com/deamwork/tg_channel_bot/fetchers"
	tb "github.com/ihciah/telebot"
)

const AboutMessage = "This is a Bot designed for syncing message(text/image/video) " +
	"from given sites to telegram channel/user/group by @ihciah.\n" +
	"Check https://github.com/ihciah/tg_channel_bot for source code and other information.\n"

type FetcherConfig struct {
	Base    f.BaseFetcher
	Twitter f.TwitterFetcher `json:"twitter"`
	Tumblr  f.TumblrFetcher  `json:"tumblr"`
	V2EX    f.V2EXFetcher
}

func (t *TelegramBot) CreateModule(moduleId int, channelId string) f.Fetcher {
	var fetcher f.Fetcher
	switch moduleId {
	case MTwitter:
		fetcher = &t.FetcherConfigs.Twitter
	case MTumblr:
		fetcher = &t.FetcherConfigs.Tumblr
	case MV2EX:
		fetcher = &t.FetcherConfigs.V2EX
	default:
		fetcher = &t.FetcherConfigs.Base
	}
	_ = fetcher.Init(t.Database, channelId)
	return fetcher
}

func (t *TelegramBot) RegisterHandler() {
	t.Bot.Handle("/about", t.handleAbout)
	t.Bot.Handle("/id", t.handleId)
	// TgBot.Bot.Handle("/example", TgBot.handle_example_fetcher_example)
	// TgBot.Bot.Handle("/v2ex", TgBot.handle_v2ex)
	t.Bot.Handle(tb.OnText, t.handleController)
	t.Bot.Handle(tb.OnPhoto, t.handlePhoto)
}

func (t *TelegramBot) handlePhoto(m *tb.Message) {
	chatID := strconv.FormatInt(m.OriginalChat.ID, 10)
	if m.OriginalChat.Type == "channel" {
		chatID = "@" + m.OriginalChat.Username
	}

	pass := false
	for _, v := range *t.Channels {
		if v.ID == chatID && authUser(m.Sender, *v.AdminUserIDs, t.Admins) {
			pass = true
			break
		}
	}
	if !pass {
		_, _ = t.Bot.Send(m.Sender, "Unauthorized.")
		return
	}

	var fetcher f.Fetcher
	if strings.Contains(m.Caption, "tumblr") {
		fetcher = new(f.TumblrFetcher)
	}
	_ = fetcher.Init(t.Database, chatID)
	_, _ = t.Bot.Send(m.Sender, fetcher.Block(m.Caption))
}

func (t *TelegramBot) handleAbout(m *tb.Message) {
	_, _ = t.Bot.Send(m.Sender, AboutMessage)
}

func (t *TelegramBot) handleId(m *tb.Message) {
	_, _ = t.Bot.Send(m.Chat, t.hGetId([]string{}, m))
}

func (t *TelegramBot) handleExampleFetcherExample(m *tb.Message) {
	var fetcher f.Fetcher = new(f.ExampleFetcher)
	_ = fetcher.Init(t.Database, "")
	_ = t.SendAll(m.Sender, fetcher.GetPushAtLeastOne(strconv.Itoa(m.Sender.ID), []string{}))
}

func (t *TelegramBot) handleV2ex(m *tb.Message) {
	var fetcher f.Fetcher = new(f.V2EXFetcher)
	_ = fetcher.Init(t.Database, "")
	_ = t.SendAll(m.Sender, fetcher.GetPushAtLeastOne(strconv.Itoa(m.Sender.ID), []string{}))
}

func (t *TelegramBot) handleController(m *tb.Message) {
	handlers := map[string]func([]string, *tb.Message) string{
		"addchannel":  t.requireSuperAdmin(t.hAddChannel),
		"delchannel":  t.requireSuperAdmin(t.hDelChannel),
		"listchannel": t.requireSuperAdmin(t.hListChannel),
		"addfollow":   t.hAddFollow,
		"delfollow":   t.hDelFollow,
		"listfollow":  t.hListFollow,
		"addadmin":    t.requireSuperAdmin(t.hAddAdmin),
		"deladmin":    t.requireSuperAdmin(t.hDelAdmin),
		"listadmin":   t.requireSuperAdmin(t.hListAdmin),
		"setinterval": t.hSetInterval,
		"goback":      t.hGoBack,
		"id":          t.hGetId,
	}

	var cmd string
	var params []string
	commands := strings.Fields(m.Text)
	if _, commandIn := handlers[commands[0]]; commandIn {
		cmd, params = commands[0], commands[1:]
		_ = t.Send(m.Sender, f.ReplyMessage{Caption: handlers[cmd](params, m)})
	} else {
		availableCommands := make([]string, 0, len(handlers))
		for c := range handlers {
			availableCommands = append(availableCommands, c)
		}
		reply := AboutMessage + "\nAlso, you can send /id to any chat to get chatid." + "\n\nUnrecognized command.\nAvailable commands: \n" + strings.Join(availableCommands, "\n")
		_ = t.Send(m.Sender, f.ReplyMessage{Caption: reply})
	}
}
