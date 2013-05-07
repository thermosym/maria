
package main

import (
	"os/exec"
	"strings"
	"log"
	"regexp"
	"fmt"
	"errors"
)

func test1() {
	err, info := avprobe("/work/1.mp4")
	if err != nil {
		log.Printf("%v", err)
	}
	log.Printf("%v", info)
}

type avprobeInfo struct {
	dur float32
	w,h int
	vcodec,acodec string
	fps int
	bitrate int
	vinfo,ainfo string
}

func avprobe(path string) (err error, info avprobeInfo) {
	out, err := exec.Command("avprobe", path).CombinedOutput()
	if err != nil {
		err = errors.New(fmt.Sprintf("avprobe: %v: %s", err, out))
		return
	}
	for _, l := range strings.Split(string(out), "\n") {
		var re *regexp.Regexp
		var ma []string

		re, _ = regexp.Compile(`Duration: (.{11})`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			var h,m,s,ms int
			fmt.Sscanf(ma[1], "%d:%d:%d.%d", &h, &m, &s, &ms)
			dur := float32(0)
			dur += float32(h)*3600
			dur += float32(m)*60
			dur += float32(s)
			dur += float32(ms)/100
			log.Printf("avprobe %s: dur %v => %f", path, ma[1], dur)
			info.dur = dur
		}

		re, _ = regexp.Compile(`Video: .* (\d+x\d+)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			var w,h int
			fmt.Sscanf(ma[1], "%dx%d", &w, &h)
			log.Printf("avprobe %s: size %v => %dx%d", path, ma[1], w, h)
			info.w = w
			info.h = h
		}

		re, _ = regexp.Compile(`bitrate: (\d+) kb/s`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			fmt.Sscanf(ma[1], "%d", &info.bitrate)
			log.Printf("avprobe %s: bitrate %v => %d", path, ma[1], info.bitrate)
		}

		re, _ = regexp.Compile(`Video: (\w+)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			info.vcodec = ma[1]
			log.Printf("avprobe %s: vcodec %s", path, info.vcodec)
		}

		re, _ = regexp.Compile(`Audio: (\w+)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			info.acodec = ma[1]
			log.Printf("avprobe %s: acodec %s", path, info.acodec)
		}

		re, _ = regexp.Compile(`Video: (.*)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			info.vinfo = ma[1]
			log.Printf("avprobe %s: vinfo %s", path, info.vinfo)
		}

		re, _ = regexp.Compile(`Audio: (.*)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			info.ainfo = ma[1]
			log.Printf("avprobe %s: ainfo %s", path, info.ainfo)
		}
	}
	return
}

