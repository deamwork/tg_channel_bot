package fetchers

import (
	"bytes"
	"errors"
	"log"

	"github.com/go-xmlpath/xmlpath"
)

type ExampleFetcher struct {
	BaseFetcher
}

func (f *ExampleFetcher) GetPush(string, []string) []ReplyMessage {
	pageUrl := "https://www.v2ex.com/i/R7yApIA5.jpeg"
	response, err := f.HTTPGet(pageUrl)
	imgUrl, err := f.parseImgPage(response)
	if err != nil {
		log.Println("Cannot do parse", err)
		return []ReplyMessage{{Err: err}}
	}
	log.Println("Image url get", imgUrl)
	reply := ReplyMessage{[]Resource{{URL: imgUrl, T: TIMAGE}}, "", nil}
	return []ReplyMessage{reply}
}

func (f *ExampleFetcher) parseImgPage(resp []byte) (string, error) {
	path := xmlpath.MustCompile("//input[@class='sls']/@value")
	root, err := xmlpath.ParseHTML(bytes.NewBuffer(resp))
	if err != nil {
		return "", err
	}
	results := make([]string, 0, 5)
	iter := path.Iter(root)
	for iter.Next() {
		results = append(results, iter.Node().String())
	}
	if len(results) >= 2 {
		return results[1], nil
	}
	return "", errors.New("unable to parse html")
}
