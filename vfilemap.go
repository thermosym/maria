
package main

import (
	"io/ioutil"
	"sync"
	"sort"
	"path/filepath"
	"os"
	"io"
	"fmt"
	"strings"
)

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

func (m vfilelist) getLiveVfile(at float32) (rv *vfile, rat float32, ridx int) {
	at = getloopat(at, m.dur)
	pos := float32(0)
	for _, v := range m.m {
		if pos + v.Dur > at {
			tmstart := pos
			for i, t := range v.Ts {
				pos += t.Dur
				if pos > at {
					return v, at-tmstart, i
				}
			}
		}
		pos += v.Dur
	}
	return nil, 0, 0
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

func vfilelistParse(c string) (m vfilemap) {
	m = vfilemap{}
	return
}



