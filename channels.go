package main

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/asdine/storm"
	f "github.com/deamwork/tg_channel_bot/fetchers"
	"github.com/ihciah/telebot"
)

const (
	MTwitter = iota
	MTumblr
	MV2EX
)

const (
	DefaultInterval         = 600
	DefaultMessageQueueSize = 6000
)

const (
	SignalExit = iota
	SignalReload
)

const (
	ChannelActionEnable = iota
	ChannelActionDisable
	ChannelActionAddAdmin
	ChannelActionDelAdmin
	ChannelActionAddFollow
	ChannelActionDelFollow
	ChannelActionUpdatePushInterval
)

type ModuleUser struct {
	Module   int
	Username string
}

type ModuleInterval struct {
	Module       int
	PushInterval int
}

type ChannelSetting struct {
	ID            string `storm:"id"`
	Enabled       bool   `storm:"index"`
	AdminUserIDs  *[]string
	Followings    *map[int][]string
	PushIntervals *map[int]int
}

type ModuleLabeler struct {
	M2s map[int]string
	S2m map[string]int
}

func (M *ModuleLabeler) Str2Module(s string) int {
	v, ok := M.S2m[s]
	if ok {
		return v
	}
	return -1
}

func (M *ModuleLabeler) Module2Str(i int) string {
	v := M.M2s[i]
	return v
}

func MakeModuleLabeler() *ModuleLabeler {
	m2s := map[int]string{
		MTwitter: "twitter",
		MTumblr:  "tumblr",
		MV2EX:    "v2ex",
	}
	s2m := make(map[string]int, len(m2s))
	for k, v := range m2s {
		s2m[v] = k
	}
	m := ModuleLabeler{M2s: m2s, S2m: s2m}
	return &m
}

func (cset *ChannelSetting) update(action int, param interface{}) {
	pint, iok := param.(ModuleInterval)
	puser, uok := param.(ModuleUser)
	newAdmin, aok := param.(string)

	switch action {
	case ChannelActionEnable:
		cset.Enabled = true
	case ChannelActionDisable:
		cset.Enabled = false
	case ChannelActionAddFollow:
		if uok {
			if cset.Followings == nil {
				followings := make(map[int][]string)
				cset.Followings = &followings
			}

			_, ok := (*cset.Followings)[puser.Module]
			if !ok {
				(*cset.Followings)[puser.Module] = make([]string, 0, 0)
			}

			for _, u := range (*cset.Followings)[puser.Module] {
				if u == puser.Username {
					return
				}
			}

			(*cset.Followings)[puser.Module] = append((*cset.Followings)[puser.Module], puser.Username)
			_, ok = (*cset.PushIntervals)[puser.Module]
			if !ok {
				(*cset.PushIntervals)[puser.Module] = DefaultInterval
			}
		}
	case ChannelActionDelFollow:
		if uok {
			for i, u := range (*cset.Followings)[puser.Module] {
				if u == puser.Username {
					(*cset.Followings)[puser.Module] = append((*cset.Followings)[puser.Module][:i], (*cset.Followings)[puser.Module][i+1:]...)
					if len((*cset.Followings)[puser.Module]) == 0 {
						delete(*cset.Followings, puser.Module)
					}
					break
				}
			}
		}
	case ChannelActionUpdatePushInterval:
		if cset.PushIntervals == nil {
			intervals := make(map[int]int)
			cset.PushIntervals = &intervals
		}

		if iok {
			(*cset.PushIntervals)[pint.Module] = pint.PushInterval
		}
	case ChannelActionAddAdmin:
		if aok {
			for _, admin := range *cset.AdminUserIDs {
				if newAdmin == admin {
					return
				}
			}
			*cset.AdminUserIDs = append(*cset.AdminUserIDs, newAdmin)
		}
	case ChannelActionDelAdmin:
		if aok {
			for i, admin := range *cset.AdminUserIDs {
				if newAdmin == admin {
					*cset.AdminUserIDs = append((*cset.AdminUserIDs)[:i], (*cset.AdminUserIDs)[i+1:]...)
					return
				}
			}
		}
	}

}

type Channel struct {
	*ChannelSetting
	DB             *storm.DB
	TgBot          *TelegramBot
	PushControl    chan int
	Chat           *telebot.Chat
	MessageControl chan int
	MessageList    chan f.ReplyMessage
}

func (c *Channel) UpdateSettings(action int, param interface{}) {
	c.update(action, param)
	_ = c.DB.Save(c.ChannelSetting)
}

func (c *Channel) PushModule(control chan int, moduleId int, followings []string, waitTime time.Duration) {
	fetcher := c.TgBot.CreateModule(moduleId, c.ID)
	for {
		log.Printf("Will check for update for module %s-%s:%s",
			c.ID, MakeModuleLabeler().Module2Str(moduleId), strings.Join(followings, ","))
		nextStart := time.After(waitTime)
		func() {
			defer func() {
				if err := recover(); err != nil {
					log.Println("Panic!", err)
				}
			}()
			for _, m := range fetcher.GetPush(c.ID, followings) {
				c.MessageList <- m
			}
		}()
		select {
		case <-control:
			log.Println("Received exit signal")
			return
		case <-nextStart:
			log.Println("Sleeping")
			continue
		}
	}
}

func (c *Channel) WaitSend() {
	for {
		timeLimit := time.After(time.Duration(3) * time.Second)
		msg := <-c.MessageList
		func() {
			defer func() {
				if err := recover(); err != nil {
					log.Println("Panic!", err)
				}
			}()
			_ = c.TgBot.Send(c.Chat, msg)
		}()

		select {
		case <-c.MessageControl:
			return
		case <-timeLimit:
			continue
		}
	}
}

func (c *Channel) Push() {
	go c.WaitSend()
	for {
		controllers := make([]chan int, 0, len(*c.Followings))
		for moduleId, followings := range *c.Followings {
			signal := make(chan int, 1)
			controllers = append(controllers, signal)
			if len(followings) == 0 {
				log.Printf("Module %d started but there's no followings.", moduleId)
			} else {
				go c.PushModule(signal, moduleId, followings, time.Duration((*c.PushIntervals)[moduleId])*time.Second)
				log.Printf("Module %d started:%s.", moduleId, strings.Join(followings, ","))
			}
		}
		select {
		case t := <-c.PushControl:
			log.Printf("Receive signal %d.", t)
			for _, cc := range controllers {
				cc <- 1
			}
			if t == SignalExit {
				return
			} else if t == SignalReload {
				continue
			}
		}
	}
}

func (c *Channel) Reload() {
	c.PushControl <- SignalReload
}

func (c *Channel) Exit() {
	c.PushControl <- SignalExit
	c.MessageControl <- SignalExit
}

func (c *Channel) Enable() {
	c.Enabled = true
	c.Reload()
}

func (c *Channel) Disable() {
	c.Enabled = false
	c.Reload()
}

func (c *Channel) AddFollowing(user ModuleUser) {
	c.UpdateSettings(ChannelActionAddFollow, user)
	c.Reload()
}

func (c *Channel) DelFollowing(user ModuleUser) {
	c.UpdateSettings(ChannelActionDelFollow, user)
	c.Reload()
}

func (c *Channel) AddAdmin(user string) {
	c.UpdateSettings(ChannelActionAddAdmin, user)
}

func (c *Channel) DelAdmin(user string) {
	c.UpdateSettings(ChannelActionDelAdmin, user)
}

func (c *Channel) UpdateInterval(interval ModuleInterval) {
	c.UpdateSettings(ChannelActionUpdatePushInterval, interval)
	c.Reload()
}

func MakeChannels(telegramBot *TelegramBot) []*Channel {
	db := telegramBot.Database
	var channelSettings []ChannelSetting
	err := db.All(&channelSettings)
	if err != nil {
		log.Fatal("Cannot read channel settings.", err)
		return []*Channel{}
	}
	var channels []*Channel
	for i := range channelSettings {
		chat, err := telegramBot.Bot.ChatByID(channelSettings[i].ID)
		if err != nil {
			log.Println("Error when start chat.", err)
			_ = db.DeleteStruct(&channelSettings[i])
			log.Printf("Chat %s deleted.\n", channelSettings[i].ID)
			continue
		}
		channels = append(channels, &Channel{&channelSettings[i], db, telegramBot, make(chan int),
			chat, make(chan int), make(chan f.ReplyMessage, DefaultMessageQueueSize)})
	}
	return channels
}

func AddChannelIfNotExists(telegramBot *TelegramBot, channelId string) (*Channel, error) {
	db := telegramBot.Database
	tx, err := db.Begin(true)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var c ChannelSetting
	e := tx.One("ID", channelId, &c)
	if e != storm.ErrNotFound {
		log.Println(e)
		return nil, errors.New("invalid channel id or channel already exists")
	}

	followings := make(map[int][]string)
	intervals := make(map[int]int)
	admins := make([]string, 0, 0)
	channelSetting := ChannelSetting{
		ID:            channelId,
		Enabled:       true,
		AdminUserIDs:  &admins,
		Followings:    &followings,
		PushIntervals: &intervals,
	}

	e = tx.Save(&channelSetting)
	if e != nil {
		log.Println(e)
		return nil, errors.New("invalid channel id or channel already exists")
	}

	chat, err := telegramBot.Bot.ChatByID(channelId)
	if err != nil {
		log.Println("Error when start chat.", err)
		return nil, errors.New("cannot create chat")
	}

	log.Println("Channel added.")
	_ = tx.Commit()

	return &Channel{&channelSetting, db, telegramBot, make(chan int), chat,
		make(chan int), make(chan f.ReplyMessage, DefaultMessageQueueSize)}, nil
}

func DelChannelIfExists(telegramBot *TelegramBot, channelId string) error {
	db := telegramBot.Database

	err := db.DeleteStruct(&ChannelSetting{ID: channelId})
	if err != nil {
		log.Println("Error when delete channel.", err)
		return err
	}

	return nil
}

func RunPusher(telegramBot *TelegramBot) {
	channels := MakeChannels(telegramBot)
	telegramBot.Channels = &channels

	for _, c := range channels {
		go c.Push()
	}
}
