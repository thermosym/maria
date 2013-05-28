
package main

import (
	"time"
	"fmt"
	"log"
	"path/filepath"
	"io"
	"os"
	"net/http"
	"strings"
	"errors"
)

type vproxyStat struct {
	op string
	stat string
	path string
	ts []tsinfo2
	curseq int
	curts int
	ist iocopyStat
	info avprobeStat
	err error
}

type vproxyCb func (st vproxyStat) error

const (
	TSNR = 30
)

func vproxyRun(prefix string, m3u8url string, cb vproxyCb, opts... interface{}) (err error) {

	debug := true

	curseq := -1
	curts := -1
	timeout := -1
	var files [TSNR]tsinfo2
	probeMode := false
	tmstart := time.Now()
	probed := false
	var curlopt string

	getTs := func (path, url string) (err error) {
		if debug {
			log.Printf("get ts %s", path)
		}
		err,_,_ = curl3(url, path,
			func (ist iocopyStat) error {
				if !probeMode {
					tmstart = time.Now()
				}
				if debug {
					log.Printf("getting %s %s %s",
							path, perstr(ist.per), speedstr(ist.speed))
				}
				cb(vproxyStat{op:"progress", ist:ist})
				return nil
			}, curlopt)
		if err != nil {
			if debug {
				log.Printf("getts: %v", err)
			}
			return
		}
		return
	}

	getAllTs := func (seq int, ts []tsinfo2) (err error) {
		var file tsinfo2
		i := curts
		for ; i < seq+len(ts) && i<curts+len(ts); {
			file = ts[i-curts]
			file.path = filepath.Join(prefix, fmt.Sprintf("%d.ts", i%TSNR))
			err = getTs(file.path, file.url)
			if err != nil {
				return
			}
			if !probed {
				var info avprobeStat
				err, info = avprobe2(file.path)
				if err != nil {
					return
				}
				cb(vproxyStat{op:"probe", info:info})
				probed = true
			}
			if probeMode && probed {
				return
			}
			files[i%TSNR] = file
			i++
		}
		return
	}

	for _, o := range opts {
		switch o.(type) {
		case string:
			if o.(string) == "probe" {
				probeMode = true
			}
			fmt.Sscanf(o.(string), "timeout=%d", &timeout)
		}
	}

	if probeMode && timeout == -1 {
		timeout = 10
	}
	if !probeMode && timeout == -1 {
		timeout = 10
	}

	curlopt = fmt.Sprintf("timeout=%d", timeout)

	for {
		if debug {
			log.Printf("fetching")
		}
		if time.Since(tmstart) > time.Duration(timeout)*time.Second {
			if probeMode {
				err = errors.New("probe timeout")
				return
			} else {
				cb(vproxyStat{op:"timeout", err:errors.New("conn timeout")})
			}
		}

		var body string
		body, err = curl(m3u8url, curlopt)
		if err != nil {
			if debug {
				log.Printf("fetch m3u8 err %v", err)
			}
			cb(vproxyStat{op:"warn", err:errors.New("fetch m3u8 err")})
			time.Sleep(time.Second)
			continue
		}

		ts, seq := parseM3u8(m3u8url, body)
		if seq == -1 {
			if debug {
				log.Printf("m3u8 seq not found")
			}
			cb(vproxyStat{op:"warn", err:errors.New("m3u8 seq not found")})
			time.Sleep(time.Second)
			continue
		}
		if len(ts) == 0 {
			if debug {
				log.Printf("no ts found m3u8 body: %s", body)
			}
			cb(vproxyStat{op:"warn", err:errors.New("no ts entries found in m3u8")})
			time.Sleep(time.Second)
			continue
		}

		//log.Printf("%s", body)
		if debug {
			log.Printf("m3u8: %s seq %d ts nr %d curseq %d curts %d",
					m3u8url, seq, len(ts), curseq, curts)
		}
		if curseq == -1 {
			curseq = seq
			curts = seq
		}
		if seq < curseq {
			curseq = seq
			curts = seq
		}
		if seq > curts {
			curseq = seq
			curts = seq
		}
		//log.Printf("%s", body)

		err = getAllTs(seq, ts)
		if probeMode {
			return
		} else {
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
		}

		curts = seq+len(ts)
		curseq = seq

		st := vproxyStat{op:"update", curts:curts, curseq:curseq}
		for j := curseq; j < curts; j++ {
			st.ts = append(st.ts, files[j%TSNR])
		}
		cb(st)

		time.Sleep(time.Second)
	}
}

func genM3u8(w io.Writer, prefix string, ts []tsinfo2, seq int) {
	if len(ts) == 0 {
		return
	}
	fmt.Fprintf(w, "#EXTM3U\n")
	fmt.Fprintf(w, "#EXT-X-TARGETDURATION:%.0f\n", tmdur2float(ts[0].Dur))
	if seq != -1 {
		fmt.Fprintf(w, "#EXT-X-MEDIA-SEQUENCE:%d\n", seq)
	}
	for _, t := range ts {
		fmt.Fprintf(w, "#EXTINF:%.0f,\n", tmdur2float(t.Dur))
		fmt.Fprintf(w, "%s%s\n", prefix, t.path)
	}
}

func testproxy3(a []string) {
	url := "http://live.gslb.letv.com/gslb?stream_id=cctv8&tag=live&ext=m3u8&sign=live_ipad"
	log.Printf("test proxying %s", url)
	vproxyRun("/tmp", url, func (st vproxyStat) error {
		switch st.op {
		case "progress":
			log.Printf("speed %s", speedstr(st.ist.speed))
		case "update":
		case "timeout":
			log.Printf("timeout")
		}
		return nil
	})
}

func testproxy2(a []string) {
	if len(a) < 2 {
		return
	}
	log.Printf("reading %s", a[1])

	type resS struct {
		err error
		line string
		info avprobeStat
		time time.Duration
	}
	res := []resS{}

	i := 0
	readLines(a[1], func (line string) error {
		arr := strings.Split(line, ",")
		if len(arr) < 2 {
			return nil
		}
		log.Printf("probing %v", arr)

		var info avprobeStat
		tmstart := time.Now()

		os.Mkdir("/tmp/1", 0777)
		err := vproxyRun("/tmp/1", arr[1], func (st vproxyStat) error {
			info = st.info
			return nil
		}, "probe", "timeout=6")

		r := resS{
			err: err,
			line: line,
			info: info,
			time: time.Since(tmstart),
		}
		res = append(res, r)
		i++

		if r.err == nil {
			log.Printf("ok response %s bitrate %s", tmdurstr(r.time), speedstr(r.info.Bitrate*1000/8))
		} else {
			log.Printf("err %v", r.err)
		}
		//if i > 30 {
		//	return errors.New("eof")
		//}
		return nil
	})
}

func testproxy1(a []string) {
//	vproxyRun("http://sw.live.cntv.cn/cctv_p2p_cctv5.m3u8")
	type stat struct {
		url string
		st vproxyStat
	}
	stats := []*stat {
		&stat{url:"http://live.gslb.letv.com/gslb?stream_id=cctv1&tag=live&ext=m3u8&sign=live_ipad"},
		&stat{url:"http://live.gslb.letv.com/gslb?stream_id=cctv2&tag=live&ext=m3u8&sign=live_ipad"},
		&stat{url:"http://live.gslb.letv.com/gslb?stream_id=cctv4&tag=live&ext=m3u8&sign=live_ipad"},
		/*
		"http://live.gslb.letv.com/gslb?stream_id=cctv5_800&tag=live&ext=m3u8&sign=live_ipad",
		"http://live.gslb.letv.com/gslb?stream_id=cctv6&tag=live&ext=m3u8&sign=live_ipad",
		"http://live.gslb.letv.com/gslb?stream_id=cctv7&tag=live&ext=m3u8&sign=live_ipad",
		*/
	}
	for i, s := range stats {
		prefix := fmt.Sprintf("/tmp/%d", i)
		os.Mkdir(prefix, 0777)
		go func(s *stat) {
			vproxyRun(prefix, s.url,
				func (st vproxyStat) error {
					s.st = st
					return nil
				},
			)
		}(s)
	}
	http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)
		log.Printf("GET %s", path)
		switch {
		case strings.HasPrefix(path, "/tmp"):
			http.ServeFile(w, r, path)
		case strings.HasPrefix(path, "/m3u8"):
			var i int
			fmt.Sscanf(path, "/m3u8/%d.m3u8", &i)
			if i >= len(stats) {
				http.Error(w, "out of bound", 404)
				return
			}
			log.Printf("gen m3u8 %v", stats[i])
			genM3u8(w, "", stats[i].st.ts, stats[i].st.curseq)
		case path == "/":
			fmt.Fprintf(w, `
				<html>
				<head>
					<script>
						function playm3u8(i) {
							document.getElementById("vid").src = i;
							document.getElementById("tips").innerHTML = i;
						}
					</script>
				</head>
				<body>
					<pre id="tips"></pre>
					<video id="vid" autoplay></video>`)
			for i, s := range stats {
				fmt.Fprintf(w, ` <button onclick="playm3u8('/m3u8/%d.m3u8')">Local %d</button>`, i, i)
				fmt.Fprintf(w, ` <button onclick="playm3u8('%s')">Remote %d</button>`, s.url, i)
			}
			fmt.Fprintf(w, `
				</body>
				</html>
			`)
		}
	})
	http.ListenAndServe(":9193", nil)
	//genM3u8(os.Stdout, "http://localhost/", vst.ts, vst.curseq)
}

