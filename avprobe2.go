
package main

import (
	"os/exec"
	"path/filepath"
	"bufio"
	"strings"
	"log"
	"regexp"
	"fmt"
	"errors"
	"time"
)

type avprobeStat struct {
	Dur time.Duration
	W,H int
	Vcodec,Acodec string
	Fps int
	Bitrate int64
	Vinfo,Ainfo string
}

func avprobe2(path string) (err error, info avprobeStat) {
	out, err := exec.Command("avprobe", path).CombinedOutput()
	if err != nil {
		err = errors.New(fmt.Sprintf("avprobe: %v: %s", err, out))
		return
	}

	debug := false

	for _, l := range strings.Split(string(out), "\n") {
		var re *regexp.Regexp
		var ma []string

		re, _ = regexp.Compile(`Duration: (.{11})`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			var h,m,s,ms int
			fmt.Sscanf(ma[1], "%d:%d:%d.%d", &h, &m, &s, &ms)
			dur := time.Duration(0)
			dur += time.Duration(h)*3600
			dur += time.Duration(m)*60
			dur += time.Duration(s)
			dur += time.Duration(ms)/100
			if debug {
				log.Printf("avprobe %s: dur %v => %f", path, ma[1], dur)
			}
			info.Dur = dur
		}

		re, _ = regexp.Compile(`Video: .* (\d+x\d+)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			var w,h int
			fmt.Sscanf(ma[1], "%dx%d", &w, &h)
			if debug {
				log.Printf("avprobe %s: size %v => %dx%d", path, ma[1], w, h)
			}
			info.W = w
			info.H = h
		}

		re, _ = regexp.Compile(`bitrate: (\d+) kb/s`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			fmt.Sscanf(ma[1], "%d", &info.Bitrate)
			if debug {
				log.Printf("avprobe %s: bitrate %v => %d", path, ma[1], info.Bitrate)
			}
		}

		re, _ = regexp.Compile(`Video: (\w+)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			info.Vcodec = ma[1]
			if debug {
				log.Printf("avprobe %s: vcodec %s", path, info.Vcodec)
			}
		}

		re, _ = regexp.Compile(`Audio: (\w+)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			info.Acodec = ma[1]
			if debug {
				log.Printf("avprobe %s: acodec %s", path, info.Acodec)
			}
		}

		re, _ = regexp.Compile(`Video: (.*)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			info.Vinfo = ma[1]
			if debug {
				log.Printf("avprobe %s: vinfo %s", path, info.Vinfo)
			}
		}

		re, _ = regexp.Compile(`Audio: (.*)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			info.Ainfo = ma[1]
			if debug {
				log.Printf("avprobe %s: ainfo %s", path, info.Ainfo)
			}
		}
	}
	return
}

type avconvStat struct {
	dur time.Duration
	per float64
	fps int
	speed int64
}

type avconvCb func (st avconvStat) error

func avconvM3u8V2(filename,outpath string, cb func (st avconvStat) error) (err error) {

	var info avprobeStat
	err, info = avprobe2(filename)
	if err != nil {
		return
	}
	if info.Dur == time.Duration(0.0) || info.Vcodec == "" || info.Acodec == "" {
		err = errors.New("avprobe invalid info")
		return
	}

	var args []string
	args = append(args, "-i", filename)
	args = append(args, "-strict", "experimental")
	if info.Vcodec == "h264" && info.Acodec == "aac" {
		args = append(args, "-acodec", "aac", "-vcodec", "copy", "-bsf", "h264_mp4toannexb")
	} else {
		args = append(args, "-acodec", "aac", "-vcodec", "libx264")
	}
	args = append(args, "-f", "mpegts", "-")

	var args2 []string
	args2 = append(args2, "-i", "-" ,"-d", "10", "-p", outpath)
	args2 = append(args2, "-m", filepath.Join(outpath, "conv.m3u8"), "-u", "/")

	cmd := exec.Command("avconv", args...)
	f, err := cmd.StderrPipe()
	if err != nil {
		return
	}
	bf := bufio.NewReader(f)

	cmd2 := exec.Command("m3u8-segmenter", args2...)
	cmd2.Stdin, err = cmd.StdoutPipe()

	err = cmd.Start()
	if err != nil {
		return
	}
	err = cmd2.Start()
	if err != nil {
		return
	}

	for {
		line, err2 := bf.ReadString('\r')
		if err2 != nil {
			log.Printf("%v", err2)
			break
		}
		log.Printf("%s", line)

		var re *regexp.Regexp
		var ma []string
		var st avconvStat
		var found bool

		re, _ = regexp.Compile(`time=(\d+.\d+)`)
		ma = re.FindStringSubmatch(line)
		if len(ma) > 1 {
			var tm time.Duration
			fmt.Sscanf(ma[1], "%d", &tm)
			st.dur = tm*time.Second
			found = true
		}

		re, _ = regexp.Compile(`fps= *(\d+)`)
		ma = re.FindStringSubmatch(line)
		if len(ma) > 1 {
			fmt.Sscanf(ma[1], "%d", &st.fps)
		}

		re, _ = regexp.Compile(`bitrate= *(\d+)`)
		ma = re.FindStringSubmatch(line)
		if len(ma) > 1 {
			fmt.Sscanf(ma[1], "%d", &st.speed)
			st.speed /= 8
		}

		if found {
			st.per = float64(st.dur)/float64(info.Dur)
			err = cb(st)
			if err != nil {
				cmd.Process.Kill()
				cmd2.Process.Kill()
				return
			}
		}
	}

	err = cmd.Wait()
	if err != nil {
		return
	}
	err = cmd2.Wait()

	return
}

