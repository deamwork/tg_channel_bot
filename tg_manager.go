package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tb "github.com/ihciah/telebot"
)

func (t *TelegramBot) hAddChannel(p []string, m *tb.Message) string {
	if len(p) != 1 {
		return "Usage: addchannel @channel_id/chat_id"
	}
	c, err := AddChannelIfNotExists(t, p[0])
	if err != nil {
		return fmt.Sprintf("Channel/Chat %s cannot be added. %s", p, err)
	}
	*t.Channels = append(*t.Channels, c)
	go c.Push()
	return fmt.Sprintf("Channel/Chat %s added.", p)
}

func (t *TelegramBot) hDelChannel(p []string, m *tb.Message) string {
	if len(p) != 1 {
		return "Usage: delchannel @channel_id/chat_id"
	}
	if err := DelChannelIfExists(t, p[0]); err != nil {
		return fmt.Sprintf("Channel/Chat %s cannot be deleted.", p)
	}
	for i, v := range *t.Channels {
		if v.ID == p[0] {
			go v.Exit()
			*t.Channels = append((*t.Channels)[:i], (*t.Channels)[i+1:]...)
			break
		}
	}
	return fmt.Sprintf("Channel/Chat %s deleted.", p)
}

func (t *TelegramBot) hAddFollow(p []string, m *tb.Message) string {
	return t.hUser(p, m, true)
}

func (t *TelegramBot) hDelFollow(p []string, m *tb.Message) string {
	return t.hUser(p, m, false)
}

func (t *TelegramBot) hUser(p []string, m *tb.Message, isAdd bool) string {
	if len(p) != 3 {
		return "Usage: addfollow/delfollow @channel_id/chat_id site(twitter/tumblr) userid"
	}
	for _, v := range *t.Channels {
		if v.ID == p[0] {
			if !authUser(m.Sender, *v.AdminUserIDs, t.Admins) {
				return "Unauthorized."
			}
			module := MakeModuleLabeler().Str2Module(p[1])
			if module == -1 {
				return "Unsupported site."
			}
			if isAdd {
				v.AddFollowing(ModuleUser{module, p[2]})
				return "Following added."
			} else {
				v.DelFollowing(ModuleUser{module, p[2]})
				return "Following deleted."
			}
		}
	}
	return "No such channel."
}

func (t *TelegramBot) hAddAdmin(p []string, m *tb.Message) string {
	return t.hAdmin(p, m, true)
}

func (t *TelegramBot) hDelAdmin(p []string, m *tb.Message) string {
	return t.hAdmin(p, m, false)
}

func (t *TelegramBot) hListAdmin(p []string, m *tb.Message) string {
	if len(p) != 1 {
		return "Usage: listadmin @channel_id/chat_id"
	}
	for _, v := range *t.Channels {
		if v.ID == p[0] {
			if len(*v.AdminUserIDs) == 0 {
				return "No admin."
			}
			return fmt.Sprintf("Admins for %s:\n%s", v.ID, strings.Join(*v.AdminUserIDs, "\n"))
		}
	}
	return "No such channel."
}

func (t *TelegramBot) hAdmin(p []string, m *tb.Message, is_add bool) string {
	if len(p) != 2 {
		return "Usage: addadmin/deladmin @channel_id/chat_id userid"
	}
	for _, v := range *t.Channels {
		if v.ID == p[0] {
			if is_add {
				v.AddAdmin(p[1])
				return "Admin added."
			} else {
				v.DelAdmin(p[1])
				return "Admin deleted."
			}
		}
	}
	return "No such channel/chat."
}

func (t *TelegramBot) hListFollow(p []string, m *tb.Message) string {
	if len(p) != 1 {
		return "Usage: listfollow @channel_id/chat_id"
	}
	for _, v := range *t.Channels {
		if v.ID == p[0] {
			if !authUser(m.Sender, *v.AdminUserIDs, t.Admins) {
				return "Unauthorized."
			}
			ret := make([]string, 0, len(*t.Channels))
			for moduleId, names := range *v.Followings {
				ret = append(ret, fmt.Sprintf("Module: %s\nUpdateInterval: %d\nFollowings:\n%s", MakeModuleLabeler().Module2Str(moduleId), (*v.PushIntervals)[moduleId], strings.Join(names, "\n")))
			}
			if len(ret) == 0 {
				return "No followings."
			}
			return strings.Join(ret, "\n\n")
		}
	}
	return "No such channel"
}

func (t *TelegramBot) hSetInterval(p []string, m *tb.Message) string {
	if len(p) != 3 {
		return "Usage: setinterval @channel_id/chat_id site N(second)"
	}
	interval, err := strconv.Atoi(p[2])
	if err != nil || interval <= 0 {
		return "Usage: setinterval @Channel/chat_id site N(second), N should be a positive number"
	}
	for _, v := range *t.Channels {
		if v.ID == p[0] {
			if !authUser(m.Sender, *v.AdminUserIDs, t.Admins) {
				return "Unauthorized."
			}
			moduleId := MakeModuleLabeler().Str2Module(p[1])
			if moduleId < 0 {
				return "Unsupported site."
			}
			v.UpdateInterval(ModuleInterval{moduleId, interval})
			return "Push Interval Updated."
		}
	}
	return "No such channel/chat"
}

func (t *TelegramBot) hListChannel(p []string, m *tb.Message) string {
	names := make([]string, 0, len(*t.Channels))
	for _, v := range *t.Channels {
		names = append(names, v.ID)
	}
	return "Channels:\n" + strings.Join(names, "\n")
}

func (t *TelegramBot) hGoBack(p []string, m *tb.Message) string {
	if len(p) != 3 {
		return "Usage: goback @channel_id/chat_id site N(second), N=0 means reset to Now."
	}
	back, err := strconv.ParseInt(p[2], 10, 64)
	if err != nil || back < 0 {
		return "Usage: goback @Channel/chat_id site N(second), N >= 0"
	}
	for _, v := range *t.Channels {
		if v.ID == p[0] {
			if !authUser(m.Sender, *v.AdminUserIDs, t.Admins) {
				return "Unauthorized."
			}
			moduleId := MakeModuleLabeler().Str2Module(p[1])
			if moduleId < 0 {
				return "Unsupported site."
			}
			fetcher := t.CreateModule(moduleId, v.ID)
			if err := fetcher.GoBack(v.ID, back); err != nil {
				return fmt.Sprintf("Error when go back. %s", err)
			}
			return fmt.Sprintf("Site %s for channel/chat %s has been set to %d seconds before.", p[1], v.ID, back)
		}
	}
	return "No such channel/chat"
}

func (t *TelegramBot) hGetId(p []string, m *tb.Message) string {
	chatId := m.Chat.ID
	chatTitle := m.Chat.Title
	userId := m.Sender.ID
	firstName := m.Sender.FirstName
	lastName := m.Sender.LastName
	username := m.Sender.Username
	return fmt.Sprintf("Hi %s %s(%s) !\nYour ID: %d\n\nChat: %s\nChatID: %d", lastName, firstName, username, userId, chatTitle, chatId)
}

func (t *TelegramBot) requireSuperAdmin(f func([]string, *tb.Message) string) func([]string, *tb.Message) string {
	return func(p []string, m *tb.Message) string {
		if authUser(m.Sender, []string{}, t.Admins) {
			log.Println("Authorized.")
			return f(p, m)
		}
		log.Println("Unauthorized", m.Sender.Username)
		return "Unauthorized user. Superadmin needed."
	}
}

func authUser(user *tb.User, adminList []string, superAdminList []string) bool {
	for _, u := range adminList {
		if user.Username == u {
			return true
		}
	}
	for _, su := range superAdminList {
		if user.Username == su {
			return true
		}
	}
	return false
}
