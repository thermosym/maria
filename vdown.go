
package main

import (
	"log"
	"time"
	"io/ioutil"
	"errors"
	"fmt"
	"strings"
	"path/filepath"
	"regexp"
	"net/url"
)

func parseM3u8(m3u8url,body string) (ts []tsinfo2, seq int) {
	lines := strings.Split(body, "\n")
	var dur float64
	re, _ := regexp.Compile(`^[^#](\S+)`)
	seq = -1
	us,_ := url.Parse(m3u8url)
	uprefix := us.Scheme+"://"+us.Host+"/"

	for _, l := range lines {
		l = strings.TrimRight(l, "\r")
		if strings.HasPrefix(l, "#EXTINF:") {
			fmt.Sscanf(l, "#EXTINF:%f", &dur)
		}
		if strings.HasPrefix(l, "#EXT-X-MEDIA-SEQUENCE:") {
			fmt.Sscanf(l, "#EXT-X-MEDIA-SEQUENCE:%d", &seq)
		}
		if re.MatchString(l) {
			turl := l
			if !strings.HasPrefix(l, "http") {
				turl = uprefix+turl
			}
			ts = append(ts, tsinfo2{
				url: turl,
				Dur: time.Duration(dur*1000)*time.Millisecond,
			})
			dur = 0
		}
	}
	return
}

type tsinfo2 struct {
	Dur time.Duration
	Size int64
	url string
	path string
}

type downloadStat struct {
	stat string
	desc string
	filename string
	per float64
	size int64
	speed int64
	ts []tsinfo2
	cur int
	dur time.Duration
}

func downloadVfile(url, path string, cb func (s downloadStat) error) (err error) {
	var desc, body string
	var m3u8url string
	var st downloadStat

	st.stat = "parsingIndex"
	cb(st)

	switch {
	case strings.HasPrefix(url, "http://v.youku.com"):
		err, m3u8url,body, desc = parseYouku(url)
	case strings.HasPrefix(url, "http://tv.sohu.com"):
		err, m3u8url,body, desc = parseSohu(url)
	default:
		err = errors.New(fmt.Sprintf("url %s can not download", url))
	}
	if err != nil {
		return
	}

	st.desc = desc
	st.stat = "parsingM3u8"
	cb(st)

	ioutil.WriteFile(filepath.Join(path, "orig.m3u8"), []byte(body), 0777)

	st.ts, _ = parseM3u8(m3u8url, body)
	for _, t := range st.ts {
		st.dur += t.Dur
	}
	if st.dur == time.Duration(0) {
		err = errors.New("m3u8 duration == 0")
		return
	}

	st.stat = "parsedM3u8"
	cb(st)

	var dur time.Duration

	for i, t := range st.ts {
		st.stat = "downloading"
		st.filename = filepath.Join(path, fmt.Sprintf("%d.ts", i))

		var size int64
		err, st.speed, size = curl3(t.url, st.filename,
		func (ist iocopyStat) (err2 error) {
			st.speed = ist.speed
			st.per = float64(dur)/float64(st.dur)
			st.per += float64(t.Dur)/float64(st.dur)*ist.per

			st2 := st
			st2.size = st.size + ist.size
			err2 = cb(st2)
			if err2 != nil {
				return
			}
			return
		})
		if err != nil {
			return
		}

		dur += t.Dur
		st.size += size
		st.cur++
		st.stat = "completeTs"
		cb(st)

		if i == 0 {
			st.stat = "firstTs"
			cb(st)
		}
	}

	st.speed = 0
	st.stat = "done"
	cb(st)
	return
}

func testDownVfile(_a []string) {
	url := "http://v.youku.com/v_show/id_XNTU0NzczOTc2_ev_2.html"
	if len(_a) > 0 {
		url = "http://tv.sohu.com/20130417/n372981909.shtml"
	}
	downloadVfile(
		url, "/tmp",
		func (st downloadStat) error {
			log.Printf("%s %v %v %d/%d",
					st.stat, speedstr(st.speed), sizestr(st.size),
					st.cur, len(st.ts),
				)
			return nil
		},
	)
}

