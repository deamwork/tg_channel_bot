package fetchers

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/asdine/storm"
	"github.com/patrickmn/go-cache"
)

type TwitterFetcher struct {
	BaseFetcher
	api               *anaconda.TwitterApi
	AccessToken       string `json:"access_token"`
	AccessTokenSecret string `json:"access_token_secret"`
	ConsumerKey       string `json:"consumer_key"`
	ConsumerSecret    string `json:"consumer_secret"`
	cache             *cache.Cache
	channelId         string
}

const (
	MaxTweetCount = "20"
)

func (f *TwitterFetcher) Init(db *storm.DB, channelId string) (err error) {
	f.DB = db.From("twitter")
	f.api = anaconda.NewTwitterApiWithCredentials(f.AccessToken, f.AccessTokenSecret, f.ConsumerKey, f.ConsumerSecret)
	f.cache = cache.New(cacheExp*time.Hour, cachePurge*time.Hour)
	f.channelId = channelId
	return
}

func (f *TwitterFetcher) getUserTimeline(user string, time int64) ([]ReplyMessage, error) {
	v := url.Values{}
	v.Set("count", MaxTweetCount)
	v.Set("screen_name", user)

	// 拉取用户timeline
	results, err := f.api.GetUserTimeline(v)
	if err != nil {
		return []ReplyMessage{}, err
	}

	// 构建用于回复message用到的消息
	ret := make([]ReplyMessage, 0, len(results))
	// 遍历时间线结果
	// ref: https://developer.twitter.com/en/docs/tweets/data-dictionary/overview/tweet-object
	for _, tweet := range results {
		createdAtTime, err := tweet.CreatedAtTime()
		if err != nil {
			continue
		}
		tweetTime := createdAtTime.Unix()
		// 跳过比给定时间早的推文
		if tweetTime < time {
			break
		}

		var msgId string
		msgId = tweet.QuotedStatusIdStr
		// 检查推文是否为评论+转推（Quoted and retweet），若独立推文则直接采用自身ID
		if msgId == "" {
			msgId = tweet.IdStr
		}
		msgId = fmt.Sprintf("%s@%s", f.channelId, msgId)
		_, found := f.cache.Get(msgId)
		f.cache.Set(msgId, true, cache.DefaultExpiration)
		if found {
			continue
		}

		resources := make([]Resource, 0, len(tweet.ExtendedEntities.Media))
		// 遍历扩展字段，找图像/视频等资源，注意扩展字段内的才是原始资源
		// ref: https://developer.twitter.com/en/docs/tweets/data-dictionary/overview/extended-entities-object
		for _, media := range tweet.ExtendedEntities.Media {
			var rType int
			var rURL string

			switch media.Type {
			case "photo":
				rType = TIMAGE
				rURL = media.Media_url_https
			case "video":
				rType = TVIDEO
				if len(media.VideoInfo.Variants) == 0 {
					continue
				}
				rURL = media.VideoInfo.Variants[0].Url
			case "animated_gif":
				rType = TVIDEO
				if len(media.VideoInfo.Variants) == 0 {
					continue
				}
				rURL = media.VideoInfo.Variants[0].Url
			}
			if rURL != "" {
				resources = append(resources, Resource{rURL, rType, rURL})
			}

		}
		ret = append(ret, ReplyMessage{resources, tweet.FullText, nil})
	}
	return ret, nil
}

func (f *TwitterFetcher) GetPush(userID string, followings []string) []ReplyMessage {
	var lastUpdate int64
	if err := f.DB.Get("last_update", userID, &lastUpdate); err != nil {
		lastUpdate = 0
	}
	ret := make([]ReplyMessage, 0, 0)
	for _, follow := range followings {
		single, err := f.getUserTimeline(follow, lastUpdate)
		if err == nil {
			ret = append(ret, single...)
		}
	}
	if len(ret) != 0 {
		_ = f.DB.Set("last_update", userID, time.Now().Unix())
	}
	return ret
}

func (f *TwitterFetcher) GoBack(userID string, back int64) error {
	now := time.Now().Unix()
	if back > now {
		return errors.New("back too long")
	}
	return f.DB.Set("last_update", userID, now-back)
}
