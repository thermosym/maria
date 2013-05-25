
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
	op string
	desc string
	filename string
	per float64
	size int64
	speed int64
	ts []tsinfo2
	cur int
	dur time.Duration
	info avprobeStat
}

type vdownCb func (s downloadStat) error

func downloadVfile(url, path string, cb vdownCb, opts... interface{}) (err error) {
	var desc, body string
	var m3u8url string
	var st downloadStat

	probeMode := false
	for _, o := range opts {
		switch o.(type) {
		case string:
			if o.(string) == "probe" {
				probeMode = true
			}
		}
	}
	curlopt := "timeout=10"

	switch {
	case strings.HasPrefix(url, "http://v.youku.com"):
		err, m3u8url,body, desc = parseYouku(url, curlopt)
	case strings.HasPrefix(url, "http://tv.sohu.com"):
		err, m3u8url,body, desc = parseSohu(url, curlopt)
	default:
		err = errors.New(fmt.Sprintf("url %s can not download", url))
	}
	if err != nil {
		return
	}

	st.desc = desc
	st.op = "desc"
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
	st.op = "ts"
	cb(st)

	errp := errors.New("probe quit")

	var dur time.Duration

	for i, t := range st.ts {
		filename := filepath.Join(path, fmt.Sprintf("%d.ts", i))

		var size int64
		err, st.speed, size = curl3(t.url, filename,
		func (ist iocopyStat) (err2 error) {
			st.speed = ist.speed
			st.per = float64(dur)/float64(st.dur)
			st.per += float64(t.Dur)/float64(st.dur)*ist.per

			st2 := st
			st2.size = st.size + ist.size
			st2.op = "progress"
			err2 = cb(st2)
			if err2 != nil {
				return
			}

			if probeMode && st2.size > 1024*100 {
				return errp
			}

			return
		}, curlopt)

		if err != nil && err != errp {
			return
		}

		dur += t.Dur
		st.size += size
		st.cur++

		if i == 0 {
			err, st.info = avprobe2(filename)
			st.op = "probe"
			cb(st)
			if err != nil {
				return
			}
			if probeMode {
				return
			}
		}
	}
	return
}

func testvdown1(_a []string) {
	url := "http://v.youku.com/v_show/id_XNTU0NzczOTc2_ev_2.html"
	if len(_a) > 0 {
		url = "http://tv.sohu.com/20130417/n372981909.shtml"
	}
	downloadVfile(
		url, "/tmp",
		func (st downloadStat) error {
			log.Printf("%v %v %d/%d",
					speedstr(st.speed), sizestr(st.size),
					st.cur, len(st.ts),
				)
			return nil
		},
	)
}

func testvdown2(a []string) {
	url := "http://v.youku.com/v_show/id_XNTU0NzczOTc2_ev_2.html"
	downloadVfile(
		url, "/tmp",
		func (st downloadStat) error {
			return nil
		},
	)
}

func testvdown3(a []string) {
	urls := []string {
		"http://groups.google.com/forum/?fromgroups#!topic/golang-china/9Tc1q01CSRU",
		"http://v.youku.com/v_show/id_XNTYxNTgyOTQw_ev_4.html",
		"http://www.youku.com",
		"http://tv.sohu.com",
		"http://www.lpfrx.com/archives/4371/",
		"http://news.youku.com/biye2013?ev=2",
		"http://v.youku.com/v_show/id_XNTYxNjcyNTE2.html",
		"http://tv.sohu.com/20130518/n376368240.shtml",
		"http://tv.sohu.com/20130525/n377041056.shtml",
	}
	for _, u := range urls {
		log.Printf("probe %s", u)
		err := downloadVfile(
			u, "/tmp",
			func (st downloadStat) error {
				switch st.op {
				case "desc":
					log.Printf("  desc %s", st.desc)
				case "probe":
					log.Printf("  info %v", st.info)
				case "ts":
					log.Printf("  dur %s", tmdurstr(st.dur))
				}
				return nil
			},
			"probe",
		)
		if err != nil {
			log.Printf("  err %v", err)
		}
	}
}

