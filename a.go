
package main

import (
	"github.com/hoisie/mustache"

	"bytes"
	"net/http"
	"crypto/sha1"
	"errors"
	"path/filepath"
	"sync"
	"net"
	"time"
	"fmt"
	"log"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"encoding/json"
	"strings"
	"sort"
)

func test1() {
	avprobe("/work/1/a.mp4")
}

func avprobe(path string) (err error, dur float32, w,h int) {
	out, err := exec.Command("ffprobe", path).CombinedOutput()
	if err != nil {
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
			dur += float32(h)*3600
			dur += float32(m)*60
			dur += float32(s)
			dur += float32(ms)/100
			log.Printf("dur %v => %f", ma[1], dur)
		}
		re, _ = regexp.Compile(`Video: .* (\d+x\d+)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			fmt.Sscanf(ma[1], "%dx%d", &w, &h)
			log.Printf("wh %v => %dx%d", ma[1], w, h)
		}
	}
	return
}

func curl(url string) (body string, err error){
	var resp *http.Response
	resp, err = http.Get(url)
	if err != nil {
		return
	}
	var bodydata []byte
	bodydata, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	body = string(bodydata)
	return
}

func curldata(url string) (body []byte, err error) {
	var resp *http.Response
	resp, err = http.Get(url)
	if err != nil {
		return
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	return
}

func (m *vfile) log(format string, v ...interface{}) {
	str := fmt.Sprintf(format, v...)
	log.Printf("vfile %s: %s", m.sha, str)
}

func (m *vfile) parseM3u8() (ts []tsinfo) {
	lines := strings.Split(m.m3u8body, "\n")
	var dur float32

	for _, l := range lines {
		if strings.HasPrefix(l, "#EXTINF:") {
			fmt.Sscanf(l, "#EXTINF:%f", &dur)
		}
		if strings.HasPrefix(l, "http") {
			ts = append(ts, tsinfo{
				url:strings.TrimRight(l, "\r"),
				Dur:dur,
			})
			dur = 0
		}
	}
	return
}

func (m *vfile) parseSohu() (err error) {

	var body string
	body, err = curl(m.Url)
	if err != nil {
		return errors.New(fmt.Sprintf("fetch index failed: %v", err))
	}

	if false {
		f, _ := os.Create("/tmp/sohu")
		data, _ := curldata(m.Url)
		f.Write(data)
		f.Close()
	}

	var re *regexp.Regexp
	re, err = regexp.Compile(`var vid="([^"]+)"`)
	ma := re.FindStringSubmatch(body)

	if len(ma) != 2 {
		return errors.New(fmt.Sprintf("sohu: cannot find vid: ma = %v", ma))
	}

	vid := ma[1]

	m3u8url := "http://hot.vrs.sohu.com/ipad"+vid+".m3u8"
	body, err = curl(m3u8url)
	if m.err != nil {
		return errors.New(fmt.Sprintf("fetch m3u8 failed: %v", err))
	}
	m.m3u8body = body

	if false {
		f, _ := os.Create("/tmp/m3u8")
		fmt.Fprintf(f, "%v", body)
		f.Close()
	}

	return
}

func (m *vfile) parseYouku() (err error) {

	var body string
	body, err = curl(m.Url)
	if err != nil {
		return errors.New(fmt.Sprintf("fetch index failed: %v", err))
	}

	var re *regexp.Regexp
	re, err = regexp.Compile(`videoId = '([^']+)'`)
	ma := re.FindStringSubmatch(body)

	if len(ma) != 2 {
		return errors.New("youku: cannot find videoId")
	}

	vid := ma[1]
	tms := fmt.Sprintf("%d", time.Now().Unix())
	m3u8url := "http://www.youku.com/player/getM3U8/vid/" + vid + "/type/hd2/ts/" + tms + "/v.m3u8"

	body, err = curl(m3u8url)
	if err != nil {
		return errors.New(fmt.Sprintf("fetch m3u8 failed: %v", err))
	}
	m.m3u8body = body
	return
}

func (m *vfile) dump() {
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	ioutil.WriteFile(filepath.Join(m.path, "info"), b, 0777)
}

func (m *vfile) load() (error) {
	b, err := ioutil.ReadFile(filepath.Join(m.path, "info"))
	if err != nil {
		return err
	}
	json.Unmarshal(b, m)
	return nil
}

func (m *vfile) hastag(tag string) bool {
	for _, s := range m.Tag {
		if s == tag {
			return true
		}
	}
	return false
}

func (m *vfile) upload(r io.Reader, length int64, ext string) {
	m.Starttm = time.Now()

	m.l.Lock()
	m.log("upload start")
	m.Stat = "uploading"
	m.Size = 0
	m.speed = 0
	m.l.Unlock()

	var err error

	shit := func () {
		m.l.Lock()
		m.log("error: %s", err)
		m.Stat = "error"
		m.err = err
		m.l.Unlock()
	}

	var f *os.File
	f, err = os.Create(filepath.Join(m.path, "a"+ext))
	if err != nil {
		shit()
		return
	}

	tmstart := time.Now()
	var n,ntx int64

	for {
		size := int64(64*1024)
		n, err = io.CopyN(f, r, size)
		if err == io.EOF {
			break
		}
		if err != nil {
			shit()
			return
		}

		m.l.Lock()
		m.Size += n
		ntx += n
		since := time.Since(tmstart)
		if since > time.Second {
			tmstart = time.Now()
			m.progress = float32(m.Size)/(float32(length)+1)
			m.speed = ntx*1000/int64(since/time.Millisecond+1)
			ntx = 0
			m.log("progress %.1f%% speed %s/s", m.progress*100, sizestr(m.speed))
		}
		m.l.Unlock()
	}

	m.l.Lock()
	m.log("done")
	m.Stat = "done"
	m.speed = 0
	m.l.Unlock()

	m.dump()
}

func (m *vfile) download(url string) {
	m.Starttm = time.Now()

	for retry := 0; retry < 10; retry++ {
		var err error
		for {
			m.log("download start")
			m.l.Lock()
			m.Stat = "parsing"
			m.l.Unlock()

			switch {
			case strings.HasPrefix(url, "http://v.youku.com"):
				err = m.parseYouku()
			case strings.HasPrefix(url, "http://tv.sohu.com"):
				err = m.parseSohu()
			default:
				return
			}
			if err != nil {
				break
			}

			ioutil.WriteFile(filepath.Join(m.path, "orig.m3u8"), []byte(m.m3u8body), 0777)

			m.l.Lock()
			m.Ts = m.parseM3u8()
			m.Dur = 0
			for _, t := range m.Ts {
				m.Dur += t.Dur
			}
			m.log("m3u8 dur %f", m.Dur)
			m.Stat = "downloading"
			m.progress = 0
			m.downN = 0
			m.l.Unlock()

			err = m.downloadAllTs()
			if err != nil {
				break
			}

			m.l.Lock()
			m.log("done")
			m.Stat = "done"
			m.l.Unlock()

			m.dump()

			return
		}
		m.l.Lock()
		m.log("error: %s", err)
		m.Stat = "error"
		m.err = err
		m.l.Unlock()
		time.Sleep(3*time.Second)
	}
	return
}

func (m *vfile) downloadTs(t *tsinfo, w io.Writer) (err error) {

	var req *http.Request
	req, err = http.NewRequest("GET", t.url, nil)
	if err != nil {
		err = errors.New(fmt.Sprintf("getts: new http req failed %v", err))
		return
	}
	req.Header = http.Header {
		"Accept" : {"*/*"},
		"User-Agent" : {"curl/7.29.0"},
	}

	var resp *http.Response
	tr := &http.Transport {
		DisableCompression: true,
		Dial: func (netw, addr string) (net.Conn, error) {
			return net.DialTimeout(netw, addr, time.Second*40)
		},
	}
	client := &http.Client{
		Transport: tr,
	}
	resp, err = client.Do(req)
	if err != nil {
		err = errors.New(fmt.Sprintf("getts: http get failed %v", err))
		return
	}

	t.Size = 0
	var n,ntx int64
	tmstart := time.Now()

	for {
		size := int64(64*1024)
		n, err = io.CopyN(w, resp.Body, size)
		t.Size += n
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			err = errors.New(fmt.Sprintf("getts: fetch failed %v", err))
			break
		}

		m.l.Lock()
		ntx += n
		m.Size += n

		if resp.ContentLength != -1 {
			m.progress = (m.downDur + float32(t.Size)/float32(resp.ContentLength)*t.Dur) /m.Dur
		} else {
			m.progress = m.downDur/m.Dur
		}

		since := time.Since(tmstart)
		if since > time.Second {
			tmstart = time.Now()
			m.speed = ntx*1000/int64(since/time.Millisecond+1)
			ntx = 0
			m.log("progress %.1f%% speed %s/s", m.progress*100, sizestr(m.speed))
		}
		m.l.Unlock()

	}

	m.l.Lock()
	m.speed = 0
	m.l.Unlock()

	return
}

func (m *vfile) downloadAllTs() (err error) {
	m.dump()
	m.downDur = 0
	for i, t := range m.Ts {
		var w *os.File
		path := filepath.Join(m.path, fmt.Sprintf("%d.ts", i))
		w, err = os.Create(path)
		if err != nil {
			err = errors.New(fmt.Sprintf("getallts: create %s failed", path))
			return
		}

		m.log("downloading ts %d/%d", i+1, len(m.Ts))

		err = m.downloadTs(&t, w)
		if err != nil {
			return
		}
		m.l.Lock()
		m.downDur += t.Dur
		m.downN++
		m.l.Unlock()
		w.Close()
	}
	return
}

func (v vfile) Statstr() string {
	stat := ""
	switch v.Stat {
	case "parsing":
		stat += "[解析中]"
	case "downloading":
		stat += fmt.Sprintf("[下载中%.1f%%]", v.progress*100)
		stat += fmt.Sprintf("[%s]", sizestr(v.Size))
		stat += fmt.Sprintf("[%s]", durstr(v.Dur))
	case "done":
		stat += "[已完成]"
		stat += fmt.Sprintf("[%s]", sizestr(v.Size))
		stat += fmt.Sprintf("[%s]", durstr(v.Dur))
	case "uploading":
		stat += fmt.Sprintf("[上传中%.1f%%]", v.progress*100)
		stat += fmt.Sprintf("[%s]", sizestr(v.Size))
		stat += fmt.Sprintf("[%s]", durstr(v.Dur))
	case "error":
		stat += "[出错]"
	case "nonexist":
		stat += "[未下载]"
	}
	return stat
}

func (v *vfile) genM3u8(w io.Writer, host string) {
	maxdur := float32(0)
	for _, t := range v.Ts {
		if t.Dur > maxdur {
			maxdur = t.Dur
		}
	}
	fmt.Fprintf(w, "#EXTM3U\n")
	fmt.Fprintf(w, "#EXT-X-TARGETDURATION:%.1f\n", maxdur)
	for i, t := range v.Ts {
		if v.Size == 0 {
			continue
		}
		fmt.Fprintf(w, "#EXTINF:%.1f,\n", t.Dur)
		fmt.Fprintf(w, "http://%s/%s/%d.ts\n", host, v.path, i)
	}
	fmt.Fprintf(w, "#EXT-X-DISCONTINUITY\n")
	fmt.Fprintf(w, "#EXT-X-ENDLIST\n")
}

func loadVfilemap() (m vfilemap) {
	m = vfilemap{}
	m.m = map[string]*vfile{}
	dirs, err := ioutil.ReadDir("vfile")
	if err != nil {
		return
	}
	for _, d := range dirs {
		v := &vfile{
			path:filepath.Join("vfile", d.Name()),
			l:&sync.Mutex{},
			sha:d.Name(),
		}
		err := v.load()
		if err == nil {
			m.m[d.Name()] = v
			v.log("loaded %s", v.Url)
			if v.Stat == "downloading" {
				go v.download(v.Url)
			}
		}
	}
	return
}

func (m vfilemap) shotsha(sha string) (r *vfile) {
	pv := m.m[sha]
	if pv == nil {
		return
	}
	pv.l.Lock()
	v := *pv
	pv.l.Unlock()
	r = &v
	return
}

func (m vfilemap) shoturl(url string) (r *vfile) {
	return m.shotsha(getsha1(url))
}

func (m vfilemap) shotall() (rm *vfilelist) {
	rm = &vfilelist{}
	for _, pv := range m.m {
		pv.l.Lock()
		v := *pv
		pv.l.Unlock()
		rm.m = append(rm.m, &v)
	}
	rm.dosum()
	sort.Sort(rm)
	return
}

func (m vfilemap) download(url string) (v *vfile) {
	sha := getsha1(url)
	v = m.shotsha(sha)
	if v != nil {
		return
	}
	v = &vfile{}
	v.sha = sha
	v.l = &sync.Mutex{}
	v.Url = url
	v.path = filepath.Join("vfile", v.sha)
	os.Mkdir(v.path, 0777)
	m.m[sha] = v
	go v.download(url)
	return
}

func (m vfilemap) upload(name string, ext string, r io.Reader, length int64) (v *vfile) {
	sha := getsha1(name)
	v = m.shotsha(sha)
	if v != nil {
		return
	}
	v = &vfile{}
	v.sha = sha
	v.l = &sync.Mutex{}
	v.path = filepath.Join("vfile", v.sha)
	v.Url = v.path
	v.Tag = []string{"upload"}
	v.Desc = name
	os.Mkdir(v.path, 0777)
	m.m[sha] = v
	v.upload(r, length, ext)
	return
}

type tsinfo struct {
	Dur float32
	Size int64
	url string
}

type vfile struct {
	Url string
	Desc string
	Tag []string
	Size int64
	Stat string
	Dur float32
	Ts []tsinfo
	Starttm time.Time

	sha string
	path string
	l *sync.Mutex
	m3u8body string
	speed int64
	downDur float32
	downN int
	progress float32
	err error
}

func testvfile() {
	urls := []string {
		"http://tv.sohu.com/20130417/n372981909.shtml",
		/*
		"http://tv.sohu.com/20130409/n372077553.shtml",
		"http://tv.sohu.com/20130407/n371829027.shtml",
		"http://tv.sohu.com/20130408/n371935984.shtml",
		"http://tv.sohu.com/20130320/n369601988.shtml",
		*/
	}
	for _, u := range urls {
		global.vfile.download(u)
	}

	time.Sleep(30*time.Second)
}

type vfilemap struct {
	m map[string]*vfile
}

type vfilelist struct {
	m []*vfile
	ntot,ndone,ndownloading,nuploading int
	speed,speed2,size int64
	dur float32
}

func (m *vfilelist) dosum() {
	m.ndone = 0
	m.ndownloading = 0
	m.nuploading = 0
	m.dur = 0
	m.speed = 0
	m.speed2 = 0
	m.size = 0
	m.ntot = len(m.m)
	for _, v := range m.m {
		switch v.Stat {
		case "done":
			m.ndone++
		case "downloading":
			m.ndownloading++
			m.speed += v.speed
		case "uploading":
			m.nuploading++
			m.speed2 += v.speed
		}
		m.dur += v.Dur
		m.size += v.Size
	}
}

func (m *vfilelist) Len() int {
	return len(m.m)
}

func (m *vfilelist) Swap(i,j int) {
	m.m[i], m.m[j] = m.m[j], m.m[i]
}

func (m *vfilelist) Less(i,j int) bool {
	return m.m[i].Url < m.m[j].Url
}

func vfilelistFromContent(c string) (m *vfilelist) {
	m = &vfilelist{}
	for _, line := range splitContent(c) {
		if strings.HasPrefix(line, "http") {
			v := global.vfile.shoturl(line)
			if v == nil {
				v = &vfile{Stat:"nonexist", Url:line}
			}
			m.m = append(m.m, v)
		}
	}
	m.dosum()
	return
}

func (m vfilelist) statstr() (s string) {
	if m.ntot == 0 {
		return "[空]"
	}
	s += fmt.Sprintf("[%s][%s]", durstr(m.dur), sizestr(m.size))
	s += fmt.Sprintf("[总数%d]", m.ntot)
	if m.ndone == m.ntot {
		s += "[全部完成]"
		return
	}
	if m.ndone > 0 {
		s += fmt.Sprintf("[已完成%d]", m.ndone)
	}
	if m.ndownloading > 0 {
		s += fmt.Sprintf("[下载中%d %s/s]", m.ndownloading, sizestr(m.speed))
	}
	if m.nuploading > 0 {
		s += fmt.Sprintf("[上传中%d %s/s]", m.nuploading, sizestr(m.speed2))
	}
	return
}

func getloopat(at, dur float32) float32 {
	n := int(at/dur)
	return at - float32(n)*dur
}

func (m vfilelist) genLiveEndM3u8(w io.Writer, host string, at float32) {

	at = getloopat(at, m.dur)
	pos := float32(0)
	for _, v := range m.m {
		if pos + v.Dur > at {
			start := 0
			for i, t := range v.Ts {
				pos += t.Dur
				start = i
				if pos > at {
					break
				}
			}

			fmt.Fprintf(w, "#EXTM3U\n")
			fmt.Fprintf(w, "#EXT-X-TARGETDURATION:%.0f\n", 30.0)
			for i := start; i < len(v.Ts); i++ {
				fmt.Fprintf(w, "#EXTINF:%.0f,\n", v.Ts[i].Dur)
				fmt.Fprintf(w, "http://%s/%s/%d.ts\n", host, v.path, i)
			}
			fmt.Fprintf(w, "#EXT-X-ENDLIST\n")

			return
		}
		pos += v.Dur
	}
}

func (m vfilelist) genLiveM3u8(w io.Writer, host string, at float32) {

	type pktS struct {
		url string
		dur float32
		end bool
		pos float32
		v *vfile
	}

	pkts := []pktS{}
	pos := float32(0)

	for _, v := range m.m {
		for i, t := range v.Ts {
			pkt := pktS{}
			pkt.v = v
			pkt.dur = t.Dur
			pkt.url = fmt.Sprintf("http://%s/%s/%d.ts", host, v.path, i)
			if i == len(v.Ts)-1 {
				pkt.end = true
			}
			pkt.pos = pos
			pos += pkt.dur
			pkts = append(pkts, pkt)
		}
	}

	nloop := int(at/m.dur)
	loopat := at - float32(nloop)*m.dur

	pktsat := 0
	for i, p := range pkts {
		if p.pos > loopat {
			break
		}
		pktsat = i
	}
	seqno := len(pkts)*nloop + pktsat

	fmt.Fprintf(w, "#EXTM3U\n")
	fmt.Fprintf(w, "#EXT-X-TARGETDURATION:%.0f\n", 30.0)
	fmt.Fprintf(w, "#EXT-X-MEDIA-SEQUENCE:%d\n", seqno)

	if false {
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "# live stream at %s\n", durstr(at))
		fmt.Fprintf(w, "# loop nr %d\n", nloop)
		fmt.Fprintf(w, "# loop at %f\n", loopat)
		fmt.Fprintf(w, "# pkts nr %d\n", len(pkts))
		fmt.Fprintf(w, "# pkts at %d\n", pktsat)
		fmt.Fprintf(w, "\n")
	}

	j := 0
	for i := pktsat; i < len(pkts); i++ {
		p := pkts[i]
		fmt.Fprintf(w, "#EXTINF:%.0f,\n", p.dur)
		fmt.Fprintf(w, "%s\n", p.url)
		if p.end {
			//fmt.Fprintf(w, "#EXT-X-DISCONTINUITY\n")
		}
		j++
		if j == 3 {
			break
		}
	}

}

func (m vfilelist) genLiveM3u8_dummy(w io.Writer, host string, at float32) {

	type pktS struct {
		url string
		dur float32
		end bool
		v *vfile
	}

	pkts := []pktS{}
	segs := [][]pktS{}
	poss := []float32{}
	pos := float32(0)

	flush := func () {
		segs = append(segs, pkts)
		poss = append(poss, pos)
		pkts = []pktS{}
	}

	for _, v := range m.m {
		for i, t := range v.Ts {
			pkt := pktS{}
			pkt.v = v
			pkt.dur = t.Dur
			pkt.url = fmt.Sprintf("http://%s/%s/%d.ts", host, v.path, i)
			if i == len(v.Ts)-1 {
				pkt.end = true
			}
			pos += pkt.dur
			pkts = append(pkts, pkt)
			if len(pkts) == 3 {
				flush()
			}
		}
	}
	flush()

	nloop := int(at/m.dur)
	loopat := at - float32(nloop)*m.dur

	segat := 0
	for i := 0; i < len(segs); i++ {
		segat = i
		if poss[i] > loopat {
			break
		}
	}
	seqno := len(segs)*nloop + segat


	fmt.Fprintf(w, "#EXTM3U\n")
	fmt.Fprintf(w, "#EXT-X-TARGETDURATION:%.0f\n", 30.0)
	fmt.Fprintf(w, "#EXT-X-MEDIA-SEQUENCE:%d\n", seqno)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "# live stream at %s\n", durstr(at))
	fmt.Fprintf(w, "# loop nr %d\n", nloop)
	fmt.Fprintf(w, "# loop at %f\n", loopat)
	fmt.Fprintf(w, "# segs nr %d\n", len(segs))
	fmt.Fprintf(w, "# segs at %d\n", segat)
	fmt.Fprintf(w, "\n")

	for _, p := range segs[segat] {
		fmt.Fprintf(w, "#EXTINF:%.0f\n", p.dur)
		fmt.Fprintf(w, "%s\n", p.url)
		if p.end {
			fmt.Fprintf(w, "#EXT-X-DISCONTINUITY\n")
		}
	}
}

func (m vfilelist) genM3u8(w io.Writer, host string, args... interface{}) {
	maxdur := float32(0)
	for _, v := range m.m {
		if v.Stat != "done" {
			continue
		}
		for _, t := range v.Ts {
			if t.Dur > maxdur {
				maxdur = t.Dur
			}
		}
	}

	debug := false

	at := float32(-1)
	if len(args) > 0 {
		if args[0].(string) == "at" {
			at = args[1].(float32)
		}
	}
	fmt.Fprintf(w, "#EXTM3U\n")
	fmt.Fprintf(w, "#EXT-X-TARGETDURATION:%.0f\n", maxdur)
	fmt.Fprintf(w, "#EXT-X-MEDIA-SEQUENCE:%d\n", 0)
	if at >= 0 {
		if debug {
			fmt.Fprintf(w, "# live stream at %s\n\n", durstr(at))
		}
	}

	cur := float32(0)
	cnt := 0

	for _, v := range m.m {
		if debug {
			fmt.Fprintf(w, "# %s%s\n", v.Statstr(), v.Url)
			if v.Stat != "done" {
				fmt.Fprintf(w, "\n")
				continue
			}
		}
		for j, t := range v.Ts {
			if v.Size == 0 {
				continue
			}
			if at >= 0 && cur > at && cnt < 3 || at < 0 {
				fmt.Fprintf(w, "#EXTINF:%.0f,\n", t.Dur)
				fmt.Fprintf(w, "http://%s/%s/%d.ts\n", host, v.path, j)
				cnt++
			}
			cur += t.Dur
		}
		if len(v.Ts) > 0 {
			fmt.Fprintf(w, "#EXT-X-DISCONTINUITY\n")
		}
		//fmt.Fprintf(w, "\n")
	}

	if at < 0 {
		fmt.Fprintf(w, "#EXT-X-ENDLIST\n")
	}
}

type menu struct {
	Desc string
	Flag string
	Type string
	Content string
	M3u8Url string
	Sub map[string]*menu

	tmstart time.Time
}

type globalS struct {
	menu *menu
	vfile vfilemap
}

var (
	global globalS
)

func (m *menu) dump(w io.Writer) {
	enc := json.NewEncoder(w)
	enc.Encode(m)
}

func (m *menu) dumptree(indent int) {
	is := ""
	for i := 0; i < indent; i++ {
		is += " "
	}
	for k, s := range m.Sub {
		log.Printf("%s%s\n", is, k)
		s.dumptree(indent+1)
	}
}

func (m *menu) load(r io.Reader) {
	dec := json.NewDecoder(r)
	dec.Decode(m)
}

func (m *menu) fillM3u8Url(host,path string) {
	m.M3u8Url = "http://"+host+"/m3u8/menu"+path+"/a.m3u8"
	if m.Type == "live" {
		m.M3u8Url += "?live=1"
	}
	log.Printf("fillm3u8 %s", path)
	for s, mc := range m.Sub {
		mc.fillM3u8Url(host, path+"/"+s)
	}
}

func (m *menu) get(path string, cb func(r,p *menu, id string)) (r *menu) {
	arr := strings.Split(strings.Trim(path, "/"), "/")
	r = m
	//log.Printf("menu get : %v", arr[0])
	if arr[0] == "" || arr[0] == "/" {
		return
	}
	for _, s := range arr {
		p := r
		r = r.Sub[s]
		if r == nil {
			return nil
		}
		if cb != nil {
			cb(r, p, s)
		}
	}
	return
}

func (m *menu) ls(path string) (r map[string]*menu) {
	p := m.get(path, nil)
	if p != nil {
		r = p.Sub
	}
	return
}

func (m *menu) statstr() string {
	return ""
}

func (m *menu) newid(a map[string]*menu) string {
	for i := 0; ; i++ {
		got := false
		id := fmt.Sprintf("%d", i)
		for k, _ := range a {
			if k == id {
				got = true
				break
			}
		}
		if !got {
			return id
		}
	}
	return ""
}

func trimContent(c string) (nc string) {
	nc = ""
	for _, s := range strings.Split(c, "\n") {
		nc += strings.Trim(s, " \t") + "\n"
	}
	return
}

func (m *menu) addEntry(path, node, desc, content, flag, Type string) (r *menu) {
	p := m.get(path, nil)
	if p == nil {
		return
	}
	if p.Flag != "dir" {
		return
	}
	if node == "" {
		node = m.newid(p.Sub)
	}
	r = p.Sub[node]
	if r == nil {
		r = &menu{Sub:map[string]*menu{}}
		p.Sub[node] = r
	}
	r.Desc = desc
	r.Flag = flag
	r.Content = trimContent(content)
	return r
}

func (m *menu) del(path string) {
	var lastp *menu
	var lastid string
	ret := m.get(path, func (r,p *menu, id string) {
		lastp = p
		lastid = id
	})
	if ret == nil {
		return
	}
	if lastp == nil {
		m.Sub = map[string]*menu{}
	} else {
		delete(lastp.Sub, lastid)
	}
	log.Printf("del %s", path)
	log.Printf(" lastp %v %s", lastp, lastid)
	m.dumptree(0)
}

func (m *menu) addDir(path, node, desc string) *menu {
	return m.addEntry(path, node, desc, "", "dir", "")
}

func (m *menu) addUrl(path, node, desc, content, Type string) *menu {
	return m.addEntry(path, node, desc, content, "url", Type)
}

func (m *menu) writeFile(filename string) {
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("%v", err)
		return
	}
	ioutil.WriteFile(filename, data, 0777)
}

func (m *menu) readFile(filename string) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Printf("%v", err)
		return
	}
	json.Unmarshal(data, m)

	m.foreach(func (r *menu) {
		if r.Type == "live" {
			r.tmstart = time.Now()
		}
	})
}

func (m *menu) foreach(cb func (r *menu)) {
	cb(m)
	for _, s := range m.Sub {
		s.foreach(cb)
	}
}

func testMenu() {
	m := &menu{Flag:"dir", Sub:map[string]*menu{}}
	m.addDir("/", "youku", "优酷视频")
	m.addDir("/", "sohu", "搜狐视频")
	m.addUrl("/youku", "movie", "优酷电影", `
		http://v.youku.com/v_show/id_XNTQzMDc1OTgw.html?f=18898121
		http://v.youku.com/v_show/id_XNTM4NDU1NTA4.html
		http://v.youku.com/v_show/id_XNTM2NDA4Nzg4.html
		http://v.youku.com/v_show/id_XNTQwOTUxODg4.html
		http://v.youku.com/v_show/id_XNTQ0MDEzMjY0.html
		http://v.youku.com/v_show/id_XNTQwMTMzODMy.html
	`, "live")
	m.addUrl("/youku", "series", "优酷电视剧", `
		http://v.youku.com/v_show/id_XNTQzNjEwMDg4.html
		http://v.youku.com/v_show/id_XNTM3NjYzNDcy.html
		http://v.youku.com/v_show/id_XNDY2NjM0MjEy.html
		http://v.youku.com/v_show/id_XNDg0NzE2NTQw.html
		http://v.youku.com/v_show/id_XNTE3MDIzMTM2.html
		http://v.youku.com/v_show/id_XNTMyODc2NzYw.html
		http://v.youku.com/v_show/id_XNTM1OTkyMTQ4.html
	`, "live")
	m.addUrl("/youku", "yule", "优酷娱乐", `
		http://v.youku.com/v_show/id_XNTM4NDU3MzEy.html
		http://v.youku.com/v_show/id_XMzUwMDU2Nzky.html
		http://v.youku.com/v_show/id_XNTIxOTcyOTIw.html
		http://v.youku.com/v_show/id_XNDgzNDU0MzIw.html
		http://v.youku.com/v_show/id_XNDg3MDIwMzI4.html
		http://v.youku.com/v_show/id_XNDc1OTEwMzQw.html
	`, "live")
	m.addUrl("/youku", "news", "优酷新闻", `
		http://v.youku.com/v_show/id_XNTQzOTkwNzEy.html?f=19173059
		http://v.youku.com/v_show/id_XNTQ0MDAxMjE2.html?f=19175120
		http://v.youku.com/v_show/id_XNTQzOTg5MTI0.html?f=19173166
		http://v.youku.com/v_show/id_XNTQzNzM3NTM2.html?f=19176152
		http://v.youku.com/v_show/id_XNTQzOTk5NzQ0.html?f=19173166
	`, "live")
	m.addUrl("/sohu", "news", "搜狐新闻", `
		http://tv.sohu.com/20130417/n372980882.shtml
		http://tv.sohu.com/20130416/n372836760.shtml/index.shtml/index.shtml
		http://tv.sohu.com/20130415/n372722202.shtml/index.shtml/index.shtml
		http://tv.sohu.com/20130417/n372966923.shtml
		http://tv.sohu.com/20130415/n372751256.shtml/index.shtml/index.shtml
	`, "live")
	m.addUrl("/sohu", "yule", "搜狐娱乐", `
		http://tv.sohu.com/20130416/n372914676.shtml
		http://tv.sohu.com/20130416/n372774957.shtml
		http://tv.sohu.com/20130415/n372768491.shtml
		http://tv.sohu.com/20130129/n364976523.shtml
		http://tv.sohu.com/20130416/n372901760.shtml
	`, "live")
	m.addUrl("/sohu", "series", "搜狐电视剧", `
		http://tv.sohu.com/20130411/n372388445.shtml
		http://tv.sohu.com/20130416/n372907125.shtml
		http://tv.sohu.com/20120915/n353219021.shtml
		http://tv.sohu.com/20121212/n360255747.shtml
		http://tv.sohu.com/20130405/n371760482.shtml
	`, "live")
	m.addUrl("/sohu", "movie", "搜狐电影", `
		http://tv.sohu.com/20130417/n372981909.shtml
		http://tv.sohu.com/20130409/n372077553.shtml
		http://tv.sohu.com/20130407/n371829027.shtml
		http://tv.sohu.com/20130408/n371935984.shtml
		http://tv.sohu.com/20130320/n369601988.shtml
	`, "live")

	m.dumptree(0)
	global.menu = m
	m.writeFile("global")
}

func tmdur2float(dur time.Duration) float32 {
	a := float32(dur)/float32(time.Second)
	return a
}

func durstr(d float32) string {
	if d < 60*60 {
		return fmt.Sprintf("%d:%.2d", int(d/60), int(d)%60)
	}
	return fmt.Sprintf("%d:%.2d:%.2d", int(d/3600), int(d/60)%60, int(d)%60)
}

func sizestr(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(size)/1024/1024)
	}
	return fmt.Sprintf("%.1fG", float64(size)/1024/1024/1024)
}

func getsha1(url string) string {
	h := sha1.New()
	io.WriteString(h, url)
	s := fmt.Sprintf("%x", h.Sum(nil))[:7]
	return s
}

func splitContent(c string) (r []string) {
	for _, l := range strings.Split(c, "\n") {
		r = append(r, strings.Trim(l, "\r\n"))
	}
	return
}

func vfilelistParse(c string) (m vfilemap) {
	m = vfilemap{}
	return
}

func pathsplit(path string, from int) string {
	a := filepath.Clean(strings.Trim(path, "/"))
	b := strings.Split(a, "/")
	if len(b) <= from {
		return ""
	} else {
		return strings.Join(b[from:], "/")
	}
}

func (v *vfile) HtmlDownOrView() string {
	if v.Stat == "nonexist" {
		return fmt.Sprintf(`<a target=_blank href="/cgi/?do=downvfile&url=%s">下载</a>`, v.Url)
	} else {
		return fmt.Sprintf(`<a target=_blank href="/vfile/%s">查看</a>`, v.sha)
	}
}

func main() {

	global.menu = &menu{Flag:"dir"}
	global.menu.readFile("global")
	global.vfile = loadVfilemap()

	if len(os.Args) >= 2 && os.Args[1] == "testv" {
		testvfile()
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "test1" {
		test1()
		return
	}

	type menuTitleS struct {
		Desc,Href string
	}

	path2titles := func (path string) (tarr []menuTitleS) {
		tarr = append(tarr, menuTitleS{"主菜单", ""})
		global.menu.get(path, func (r,p *menu, id string) {
			tarr = append(tarr, menuTitleS{r.Desc, ""})
		})
		return tarr
	}

	path2title := func (path string) string {
		tarr := []string{}
		global.menu.get(path, func (r,p *menu, id string) {
			tarr = append(tarr, r.Desc)
		})
		if len(tarr) == 0 {
			return "根目录"
		} else {
			return strings.Join(tarr, " / ")
		}
	}

	pathup := func (path string) string {
		arr := strings.Split(path, "/")
		if len(arr) <= 1 {
			return ""
		}
		return strings.Join(arr[0:len(arr)-1], "/")
	}

	renderIndex := func (w io.Writer, body string) {
		s := mustache.RenderFile("tpl/index.html", map[string]string{"body": body})
		fmt.Fprintf(w, "%s", s)
	}

	listvfile := func (m *vfilelist) string {
		return mustache.RenderFile("tpl/listVfile.html",
			map[string]interface{} {
				"list": m.m,
				"statstr": m.statstr(),
			})
	}

	menuPage := func (w io.Writer, path string) {
		m := global.menu.get(path, nil)
		if m == nil {
			return
		}

		title := "编辑菜单: " + path2title(path)
		titles := path2titles(path)
		titlelast := ""
		if len(titles) > 0 {
			n := len(titles)
			titlelast = titles[n-1].Desc
			titles = titles[0:n-1]
		}

		type btn struct {
			Href, Title string
		}
		btns := []btn{}
		btns2 := []btn{}

		if path != "" {
			btns = append(btns, btn{"/menu/"+pathup(path), "上一级目录"})
		}
		if m.Flag == "dir" {
			btns = append(btns, btn{"/adddir/"+path, "添加目录"})
			btns = append(btns, btn{"/addvid/"+path, "添加视频"})
			if path != "" {
				btns = append(btns, btn{"/editdir/"+path, "修改"})
			}
		}
		btns = append(btns, btn{"/del/"+path, "删除"})

		type menuH struct {
			Tstr,Path,Desc string
		}
		mharr := []menuH{}
		liststr := ""

		at := float32(0)
		elapsed := float32(0)

		if m.Flag == "dir" {
			marr := global.menu.ls(path)
			for k, s := range marr {
				var tstr string
				switch s.Flag {
				case "dir":
					tstr = "目录"
				case "url":
					tstr = "视频"
				}
				switch s.Type {
				case "live":
					tstr = "直播"
				case "ondemand":
					tstr = "点播"
				}
				mharr = append(mharr, menuH{tstr,"/menu/"+path+"/"+k, s.Desc})
			}
		} else {
			btns2 = append(btns2, btn{"/editvid/"+path, "编辑"})
			btns2 = append(btns2, btn{"/m3u8/menu/"+path, "查看m3u8"})
			btns2 = append(btns2, btn{"/play/menu/"+path, "播放m3u8"})
			btns2 = append(btns2, btn{"/cgi/"+path+"/?do=downAllVfile", "下载全部"})

			var list *vfilelist

			if m.Content != "" {
				list = vfilelistFromContent(m.Content)
			}

			if list == nil || len(list.m) == 0 {
				liststr = `<p>[空]</p>`
			} else {
				liststr = listvfile(list)
			}

			if m.Type == "live" && list != nil && list.dur > 0 {
				elapsed = tmdur2float(time.Since(m.tmstart))
				at = elapsed - list.dur*float32(int(elapsed/list.dur))
			}
		}

		renderIndex(w,
			mustache.RenderFile("tpl/menuPage.html", map[string]interface{} {
				"btns": btns,
				"btns2": btns2,
				"isDir": m.Flag == "dir",
				"listEmpty": len(mharr) == 0,
				"list": mharr,
				"liststr": liststr,
				"title": title,
				"titles": titles,
				"titlelast": titlelast,
				"isLive": m.Type == "live",
				"tmelapsed": durstr(elapsed),
				"tmat": durstr(at),
			}))
	}

	editdirPage := func (w io.Writer, path string, op string) {
		m := global.menu.get(path, nil)
		title := ""
		desc := ""
		if op == "edit" {
			if m == nil {
				return
			}
			title = "修改目录"
			desc = m.Desc
		} else {
			title = "添加目录"
		}

		renderIndex(w,
			mustache.RenderFile("tpl/editDir.html", map[string]interface{} {
				"title": title,
				"path": path2title(path),
				"action": fmt.Sprintf("/do_%sdir/%s", op, path),
				"desc": desc,
				"backurl": fmt.Sprintf("/menu/%s", path),
			}))
	}

	editvidPage := func (w io.Writer, path string, op string) {
		m := global.menu.get(path, nil)
		content := ""
		title := ""
		desc := ""
		types := []*struct {
			Value,Checked,Desc string
		}{
			{Value:"live", Desc:"直播"},
			{Value:"ondemand", Desc:"点播"},
		}
		for i, t := range types {
			if t.Value == m.Type {
				types[i].Checked = "checked"
			}
		}

		if op == "edit" {
			if m == nil {
				return
			}
			title = "修改视频"
			content = m.Content
			desc = m.Desc
		} else {
			title = "添加视频"
		}

		renderIndex(w,
			mustache.RenderFile("tpl/editVid.html", map[string]interface{} {
				"title": title,
				"path": path2title(path),
				"action": fmt.Sprintf("/do_%svid/%s", op, path),
				"desc": desc,
				"content": content,
				"backurl": fmt.Sprintf("/menu/%s", path),
				"types": types,
			}))
	}

	doUpload := func (r *http.Request, path string) {
		log.Printf("upload: newfile %s", path)

		mr, err := r.MultipartReader()
		if err != nil {
			return
		}
		length := r.ContentLength
		part, err := mr.NextPart()
		if err != nil {
			return
		}

		filename := part.FileName()
		ext := filepath.Ext(filename)
		log.Printf("upload: newfile filename %s ext %s", filename, ext)
		global.vfile.upload(path, ext, part, length)
	}

	cgipage := func (w http.ResponseWriter, r *http.Request, path string) {
		log.Printf("cgi: path %s", path)
		if strings.HasPrefix(path, "upload") {
			doUpload(r, pathsplit(path, 1))
			return
		}
		switch r.FormValue("do") {
		case "downAllVfile":
			m := global.menu.get(path, nil)
			if m == nil || m.Flag != "url" {
				return
			}
			for _, line := range splitContent(m.Content) {
				if strings.HasPrefix(line, "http") {
					global.vfile.download(line)
				}
			}
			http.Redirect(w, r, fmt.Sprintf("/menu/%s", path), 302)
		case "downvfile":
			url := r.FormValue("url")
			v := global.vfile.download(url)
			http.Redirect(w, r, fmt.Sprintf("/vfile/%s", v.sha), 302)
		}

	}

	doedit := func (w http.ResponseWriter, r *http.Request, path string, op string, flag string) {
		if r.FormValue("desc") == "" {
			fmt.Fprintf(w, `<p>标题不能为空 [<a href="/menu/%s">返回</a>]</p>`, path)
			return
		}
		var m *menu
		if op == "add" {
			if flag == "url" {
				m = global.menu.addUrl(path, "", "", "", r.FormValue("type"))
			} else {
				m = global.menu.addDir(path, "", "")
			}
		} else {
			m = global.menu.get(path, nil)
		}
		if m == nil {
			return
		}
		m.Desc = r.FormValue("desc")
		m.Content = r.FormValue("content")
		m.Type = r.FormValue("type")
		global.menu.writeFile("global")

		if true {
			http.Redirect(w, r, fmt.Sprintf("/menu/%s", path), 302)
		} else {
			fmt.Fprintf(w, `<p>修改成功 [<a href="/menu/%s">返回</a>]</p>`, path)
		}
	}

	dodel := func (w io.Writer, path string) {
		global.menu.del(path)
		fmt.Fprintf(w, `<p>删除成功 [<a href="/menu/%s">返回</a>]</p>`, pathup(path))
		global.menu.writeFile("global")
	}

	trydel := func (w io.Writer, path string) {
		m := global.menu.get(path, nil)
		if m == nil {
			return
		}
		fmt.Fprintf(w, "<p>确认删除 '" + path2title(path) + "' ?</p>")
		fmt.Fprintf(w, `<a href="/do_del/%s">确定</a> | `, path)
		fmt.Fprintf(w, `<a href="/menu/%s">返回</a>`, path)
	}

	vfilepage := func (w http.ResponseWriter, r *http.Request, path string) {
		v := global.vfile.shotsha(getsha1(path))
		if v != nil {
			http.Redirect(w, r, fmt.Sprintf("/vfile/%s", getsha1(path)), 302)
			return
		}
		v = global.vfile.shotsha(path)
		if v == nil {
			return
		}

		renderIndex(w,
			mustache.RenderFile("tpl/viewVfile.html", map[string]interface{} {
				"url": v.Url,
				"statstr": v.Statstr(),
				"path": path,
				"starttm": v.Starttm,
				"isDownloading": v.Stat == "downloading",
				"isUploading": v.Stat == "uploading",
				"isError": v.Stat == "error",
				"progress": fmt.Sprintf("%.1f%%", v.progress*100),
				"speed": fmt.Sprintf("%s/s", sizestr(v.speed)),
				"tsTotal": len(v.Ts),
				"tsDown": v.downN,
				"error": fmt.Sprintf("%v", v.err),
			}))
	}

	manvfile := func (w io.Writer, path string) {
		list := global.vfile.shotall()
		s := listvfile(list)
		renderIndex(w, s)
	}

	vfileUpload := func (w io.Writer, path string) {
		renderIndex(w, mustache.RenderFile("tpl/vfileUpload.html", map[string]interface{}{}))
	}

	vfileM3u8 := func (w http.ResponseWriter, wr io.Writer, sha string, host string) {
		log.Printf("vfilem3u8: %s", sha)
		v := global.vfile.shotsha(sha)
		if v == nil {
			http.Error(w, "not found", 404)
			return
		}
		v.genM3u8(wr, host)
	}

	menuM3u8 := func (r *http.Request, w http.ResponseWriter, wr io.Writer, path,host string) {
		log.Printf("menum3u8 %s", path)
		m := global.menu.get(path, nil)
		if m == nil || m.Flag != "url" {
			http.Error(w, "not found", 404)
			return
		}
		list := vfilelistFromContent(m.Content)
		at := r.FormValue("at")
		live := r.FormValue("live")
		liveend := r.FormValue("liveend")
		switch {
		case at != "":
			fat := float32(0)
			fmt.Sscanf(at, "%f", &fat)
			list.genM3u8(w, host, "at", fat)
		case live != "":
			fat := tmdur2float(time.Since(m.tmstart))
			list.genLiveM3u8(w, host, fat)
		case liveend != "":
			fat := tmdur2float(time.Since(m.tmstart))
			list.genLiveEndM3u8(w, host, fat)
		default:
			list.genM3u8(w, host)
		}
	}

	sampleM3u8Starttm := time.Now()

	sampleM3u8 := func (r *http.Request, w io.Writer, host,path string) {
		log.Printf("samplem3u8 %s", path)
		list := global.vfile.shotall()
		list2 := vfilelist{}
		for _, v := range list.m {
			v.Ts = v.Ts[0:1]
			list2.m = append(list2.m, v)
			list2.dur += v.Ts[0].Dur
		}
		at := tmdur2float(time.Since(sampleM3u8Starttm))
		list2.genLiveM3u8(w, host, at)
	}

	playM3u8 := func (w io.Writer, url string) {
		log.Printf("playm3u8 %s", url)
		fmt.Fprintf(w, "<html>")
		fmt.Fprintf(w, "<body>")
		fmt.Fprintf(w, `<video src="%s" autoplay></video>`, url)
		fmt.Fprintf(w, "</body>")
		fmt.Fprintf(w, "</html>")
	}

	jsonMenu := func (w io.Writer, host,path string) {
		m := global.menu.get(path, nil)
		if m == nil {
			return
		}
		log.Printf("json menu: %s", path)
		m.fillM3u8Url(host, path)
		enc := json.NewEncoder(w)
		enc.Encode(m)
	}

	jsonCallback := func (w io.Writer, host,path string) {
		type S struct {
			Type string
			V interface{}
		}
		m := global.menu.get(path, nil)
		if m == nil {
			return
		}
		m.fillM3u8Url(host, path)
		s := S{"menu", m}

		enc := json.NewEncoder(w)
		enc.Encode(s)
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)
		log.Printf("GET %s", path)
		dir, file := filepath.Split(path)
		ext := filepath.Ext(file)
		switch ext {
		case ".ts", ".html", ".css", ".js":
			http.ServeFile(w, r, path[1:])
		case ".m3u8":
			path = dir
		}

		var bs bytes.Buffer
		oneshot := func () {
			w.Header().Add("Content-Length", fmt.Sprintf("%d", (bs.Len())))
			w.Write(bs.Bytes())
		}

		switch {
		case path == "/":
			http.Redirect(w, r, "/menu", 302)

		case strings.HasPrefix(path, "/menu"):
			menuPage(w, pathsplit(path, 1))

		case strings.HasPrefix(path, "/m3u8/vfile"):
			vfileM3u8(w, &bs, pathsplit(path, 2), r.Host)
			oneshot()
		case strings.HasPrefix(path, "/m3u8/menu"):
			menuM3u8(r, w, &bs, pathsplit(path, 2), r.Host)
			oneshot()
		case strings.HasPrefix(path, "/m3u8/sample"):
			sampleM3u8(r, &bs, r.Host, pathsplit(path, 2))
			oneshot()

		case strings.HasPrefix(path, "/json/menu"):
			jsonMenu(w, r.Host, pathsplit(path, 2))
		case strings.HasPrefix(path, "/json/callback"):
			jsonCallback(w, r.Host, pathsplit(path, 2))

		case strings.HasPrefix(path, "/play"):
			playM3u8(w, "/m3u8/"+pathsplit(path,1)+"/a.m3u8?"+r.URL.RawQuery)
		case strings.HasPrefix(path, "/addvid"):
			editvidPage(w, pathsplit(path, 1), "add")
		case strings.HasPrefix(path, "/adddir"):
			editdirPage(w, pathsplit(path, 1), "add")
		case strings.HasPrefix(path, "/editvid"):
			editvidPage(w, pathsplit(path, 1), "edit")
		case strings.HasPrefix(path, "/editdir"):
			editdirPage(w, pathsplit(path, 1), "edit")
		case strings.HasPrefix(path, "/do_editdir"):
			doedit(w, r, pathsplit(path, 1), "edit", "dir")
		case strings.HasPrefix(path, "/do_adddir"):
			doedit(w, r, pathsplit(path, 1), "add", "dir")
		case strings.HasPrefix(path, "/do_editvid"):
			doedit(w, r, pathsplit(path, 1), "edit", "url")
		case strings.HasPrefix(path, "/do_addvid"):
			doedit(w, r, pathsplit(path, 1), "add", "url")
		case strings.HasPrefix(path, "/del"):
			trydel(w, pathsplit(path, 1))
		case strings.HasPrefix(path, "/do_del"):
			dodel(w, pathsplit(path, 1))
		case strings.HasPrefix(path, "/manv/upload"):
			vfileUpload(w, pathsplit(path, 2))
		case strings.HasPrefix(path, "/manv"):
			manvfile(w, pathsplit(path, 1))
		case strings.HasPrefix(path, "/vfile"):
			vfilepage(w, r, pathsplit(path, 1))
		case strings.HasPrefix(path, "/cgi"):
			cgipage(w, r, pathsplit(path, 1))
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	})
	err := http.ListenAndServe(":9191", nil)
	if err != nil {
		log.Printf("%v", err)
	}
}

