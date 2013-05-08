
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

type avconvInfo struct {
	time float32
	per float32
	fps int
	kbps int
}

func avconvM3u8(filename,outpath string, cb func (info avconvInfo)) (err error) {

	var info avprobeInfo
	err, info = avprobe(filename)
	if err != nil {
		return
	}
	if info.dur == 0.0 || info.vcodec == "" || info.acodec == "" {
		err = errors.New("avprobe invalid info")
		return
	}

	var args []string
	args = append(args, "-i", filename)
	args = append(args, "-strict", "experimental")
	if info.vcodec == "h264" && info.acodec == "aac" {
		args = append(args, "-acodec", "aac", "-vcodec", "copy", "-bsf", "h264_mp4toannexb")
	} else {
		args = append(args, "-acodec", "aac", "-vcodec", "libx264")
	}
	args = append(args, "-f", "mpegts", "-")

	var args2 []string
	args2 = append(args2, "-i", "-" ,"-d", "10", "-p", outpath)
	args2 = append(args2, "-m", filepath.Join(outpath, "a.m3u8"), "-u", "/")

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
		var info2 avconvInfo

		re, _ = regexp.Compile(`time=(\d+.\d+)`)
		ma = re.FindStringSubmatch(line)
		if len(ma) > 1 {
			fmt.Sscanf(ma[1], "%f", &info2.time)
		}

		re, _ = regexp.Compile(`fps= *(\d+)`)
		ma = re.FindStringSubmatch(line)
		if len(ma) > 1 {
			fmt.Sscanf(ma[1], "%d", &info2.fps)
		}

		re, _ = regexp.Compile(`bitrate= *(\d+)`)
		ma = re.FindStringSubmatch(line)
		if len(ma) > 1 {
			fmt.Sscanf(ma[1], "%d", &info2.kbps)
		}

		if info2.time != 0.0 {
			info2.per = info2.time/info.dur
			cb(info2)
		}
	}

	err = cmd.Wait()
	if err != nil {
		return
	}
	err = cmd2.Wait()

	return
}

func testavconv() {
	err := avconvM3u8("/var/www/0.rmvb", "/tmp/hls2", func (info avconvInfo) {
		log.Printf("conv: %v", info)
	})
	log.Printf("%v", err)
}

func testcmd() {
	ss, _ := exec.Command("ls | wc").Output()
	log.Printf("%s", string(ss))
}

