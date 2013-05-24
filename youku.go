
package main

import (
	"regexp"
	"errors"
	"fmt"
	"time"
)

func parseYouku(url string) (err error, m3u8url,body string, desc string) {

	var ma []string
	var re *regexp.Regexp

	body, err = curl(url)
	if err != nil {
		err = errors.New(fmt.Sprintf("curl index failed: %v", err))
		return
	}

	re, err = regexp.Compile(`<meta name="title" content="([^"]+)">`)
	ma = re.FindStringSubmatch(body)
	if len(ma) >= 2 {
		desc = ma[1]
	}

	re, err = regexp.Compile(`videoId = '([^']+)'`)
	ma = re.FindStringSubmatch(body)

	if len(ma) != 2 {
		err = errors.New("cannot find videoId")
		return
	}

	vid := ma[1]
	tms := fmt.Sprintf("%d", time.Now().Unix())
	m3u8url = "http://www.youku.com/player/getM3U8/vid/" + vid + "/type/hd2/ts/" + tms + "/v.m3u8"

	body, err = curl(m3u8url)
	if err != nil {
		err = errors.New(fmt.Sprintf("curl m3u8 failed: %v", err))
		return
	}

	return
}


