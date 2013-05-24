
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
)

type vproxyStat struct {
	op string
	path string
	ts []tsinfo2
	curseq int
	curts int
	drop int
}

type vproxyCb func (st vproxyStat)

const (
	TSNR = 30
)

func vproxyRun(prefix string, m3u8url string, cb vproxyCb) {

	gotFirst := false

	getTs := func (path, url string) {
		log.Printf("get ts %s", path)
		err,speed,size := curl3(url, path,
			func (ist iocopyStat) error {
				log.Printf("getting %s %.1f%%", path, ist.per*100)
				return nil
			})
		if err != nil {
			log.Printf("%v", err)
		} else {
			if false {
				log.Printf("speed %s size %s", speedstr(speed), sizestr(size))
			}
			//_, info := avprobe2(path)
			//log.Printf("%v", info)
			if !gotFirst {
				cb(vproxyStat{op:"firstTs", path:path})
				gotFirst = true
			}
		}
	}

	drop := 0
	curseq := -1
	curts := -1
	var files [TSNR]tsinfo2

	for {
		body, err := curl(m3u8url)
		if err != nil {
			log.Printf("fetch m3u8 err")
			time.Sleep(time.Second)
			continue
		}
		ts, seq := parseM3u8(m3u8url, body)
		if seq == -1 {
			log.Printf("m3u8 seq not found")
			time.Sleep(time.Second)
			continue
		}
		if len(ts) == 0 {
			log.Printf("no ts found m3u8 body: %s", body)
			time.Sleep(time.Second)
			continue
		}
		//log.Printf("%s", body)
		log.Printf("m3u8: %s seq %d ts nr %d curseq %d curts %d drop %d",
				m3u8url, seq, len(ts), curseq, curts, drop)
		if curseq == -1 {
			curseq = seq
			curts = seq
			time.Sleep(time.Second)
			continue
		}
		if seq < curseq {
			// shit 
			curseq = seq
			curts = seq
			time.Sleep(time.Second)
			continue
		}
		if seq > curts {
			// drop
			drop += seq - curts
			curseq = seq
			curts = seq
		}
		//log.Printf("%s", body)

		var file tsinfo2

		i := curts
		for ; i < seq+len(ts) && i<curts+len(ts); {
			file = ts[i-curts]
			file.path = filepath.Join(prefix, fmt.Sprintf("%d.ts", i%TSNR))
			if true {
				getTs(file.path, file.url)
			}
			files[i%TSNR] = file
			i++
		}
		curts = i
		curseq = seq

		st := vproxyStat{op:"update", curts:curts, curseq:curseq, drop:drop}
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
				func (st vproxyStat) {
					s.st = st
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

