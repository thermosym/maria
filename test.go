
package main

import (
	"time"
	"net/http"
	"io"
	"log"
)

var (
	sampleM3u8Starttm = time.Now()
)

func sampleM3u8 (r *http.Request, w io.Writer, host,path string) {
	list := global.vfile.shotall()
	list2 := vfilelist{}
	for _, v := range list.m {
		v.Ts = v.Ts[0:1]
		v.Dur = v.Ts[0].Dur
		list2.m = append(list2.m, v)
		list2.dur += v.Ts[0].Dur
	}
	at := tmdur2float(time.Since(sampleM3u8Starttm))
	log.Printf("samplem3u8 %s at %s", path, durstr(at))
	list2.genLiveEndM3u8(w, host, at)
}

