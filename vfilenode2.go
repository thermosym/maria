
package main

import (
	"io"
	"os"
	"errors"
	"time"
	"strings"
	"fmt"
	"log"
	"sync"
	"path/filepath"
)

type vfileV2Node struct {
	Stat string
	Name string
	Type string
	Src string
	Desc string
	Filename string
	Ts []tsinfo2
	Starttm time.Time
	Info avprobeStat

	Size int64
	TotSize int64
	Dur time.Duration
	Per float64
	Speed int64

	name string
	path string

	l *sync.Mutex

	forceStop bool
	err error
}

type vfileV2NodeInfo struct {
	Stat string
	Desc string
	Type string
	Durstr string
	Geo string
	Perstr string
	Speedstr string
	Sizestr string
}

func (v *vfileV2Node) info() (info vfileV2NodeInfo) {
	info.Stat = v.Stat
	info.Type = v.Type
	if v.Dur > 0.0 {
		info.Durstr = tmdurstr(v.Dur)
	}
	if v.Info.W != 0 {
		info.Geo = fmt.Sprintf("%dx%d", v.Info.W, v.Info.H)
	}
	info.Type = v.Type
	if v.Size != 0 {
		info.Sizestr = sizestr(v.Size)
	}
	if v.TotSize != 0 {
	}
	if v.Speed > 0 {
		info.Speedstr = speedstr(v.Speed)
	}
	if v.Per > 0.0 {
		info.Perstr = perstr(v.Per)
	}
	info.Desc = v.Desc
	return
}

func (m *vfileV2Node) log(format string, v ...interface{}) {
	str := fmt.Sprintf(format, v...)
	log.Printf("vfile %s: %s", m.name, str)
}


func vfileFromPath(path string) (v *vfileV2Node) {
	v = &vfileV2Node{}
	v.path = path
	v.l = &sync.Mutex{}
	loadJson(filepath.Join(path, "info"), v)
	return
}

func (m *vfileV2Node) save() {
	m.l.Lock()
	defer m.l.Unlock()
	saveJson(filepath.Join(m.path, "info"), m)
}

func (m *vfileV2Node) stopAndRemove() {
	m.l.Lock()
	defer m.l.Unlock()
	m.forceStop = true
	os.RemoveAll(m.path)
}

func (m *vfileV2Node) modify(args form) (err error) {
	m.l.Lock()
	defer m.l.Unlock()
	str, ok := args.str2("desc")
	if ok {
		if str == "" {
			err = errors.New("desc is empty")
			return
		}
		m.Desc = str
	}
	return
}

func uploadVfileConv(
	filename,path string,
	r io.Reader,
	length int64,
	cb ioCopyCb,
	cb3 avconvCb,
	) (err error, size int64) {

	var w io.Writer
	w, err = os.Create(path)
	if err != nil {
		err = errors.New(fmt.Sprintf("create %s failed", path))
		return
	}

	err, _, size = ioCopy(r, length, w, cb, nil)
	if err != nil {
		err = errors.New(fmt.Sprintf("upload %s failed: %s", filename, err))
		return
	}

	err = avconvM3u8V2(filename, path, cb3)
	if err != nil {
		err = errors.New(fmt.Sprintf("avconv %s failed: %s", filename, err))
	}

	return
}

func (v *vfileV2Node) _upload(filename string, r io.Reader, length int64) {
	v.l.Lock()
	v.Stat = "connecting"
	v.Type = "upload"
	v.forceStop = false
	v.Filename = filename
	v.TotSize = length
	v.l.Unlock()

	iocopy := func (st iocopyStat) (err error) {
		v.l.Lock()
		defer v.l.Unlock()
		if v.forceStop {
			return errors.New("user force stop uploading")
		}
		v.Stat = "uploading"
		v.Speed = st.speed
		v.Per = st.per
		v.Size = st.size
		return
	}

	avconv := func (st avconvStat) (err error) {
		v.l.Lock()
		defer v.l.Unlock()
		if v.forceStop {
			return errors.New("user force stop avconv")
		}
		v.Stat = "avconving"
		v.Speed = st.speed
		v.Per = st.per
		return
	}

	err, size := uploadVfileConv(
		filepath.Join(v.path, filename), v.path, r, length,
		iocopy, avconv,
	)

	v.l.Lock()
	if err != nil {
		v.Stat = "error"
		v.err = err
	} else {
		v.Stat = "done"
		v.Size = size
		v.save()
	}
	v.l.Unlock()
}

func vfileNewUpload(filename string, r io.Reader, length int64, path string) (v *vfileV2Node) {
	v = &vfileV2Node{}
	v.path = path
	v.l = &sync.Mutex{}
	os.Mkdir(v.path, 0777)

	go v._upload(filename, r, length)

	return
}

func (v *vfileV2Node) _download(url string) {
	v.l.Lock()
	v.Stat = "connecting"
	v.Type = "download"
	v.Src = url
	v.forceStop = false
	v.log("download start")
	v.l.Unlock()

	err := downloadVfile(url, v.path, func (st downloadStat) (err error) {
		v.l.Lock()
		defer v.l.Unlock()
		if v.forceStop {
			return errors.New("user force stop")
		}
		switch st.op {
		case "desc":
			v.Desc = st.desc
		case "ts":
			v.Ts = st.ts
		case "progress":
			v.Stat = "downloading"
			v.Size = st.size
			v.Dur = st.dur
			v.Speed = st.speed
			v.Per = st.per
			v.log("download %s", perstr(st.per))
		case "probe":
			v.Info = st.info
		}
		return err
	})

	v.l.Lock()
	if err != nil {
		v.Stat = "error"
		v.err = err
		v.log("download error %s", err)
	} else {
		v.Stat = "done"
		v.log("download done")
		v.save()
	}
	v.l.Unlock()
}

func vfileNewDownload(url, path string) (v *vfileV2Node, err error) {
	if !strings.Contains(url, "youku") && !strings.Contains(url, "sohu") {
		err = errors.New("url invalid")
		return
	}

	v = &vfileV2Node{}
	v.path = path
	v.l = &sync.Mutex{}
	os.Mkdir(v.path, 0777)

	go v._download(url)

	return
}

func vfileNewVProxy(url, path string) (v *vfileV2Node) {
	v = &vfileV2Node{}
	v.path = path
	v.l = &sync.Mutex{}
	os.Mkdir(path, 0777)

	go func() {
		v.l.Lock()
		v.Stat = "connecting"
		v.Type = "vproxy"
		v.path = path
		v.l.Unlock()

		err := vproxyRun(path, url, func (st vproxyStat) (err error) {
			v.l.Lock()
			defer v.l.Unlock()
			if v.forceStop {
				err = errors.New("user force stop")
				return
			}
			if st.op == "probe" {
				v.Info = st.info
			}
			if st.op == "progress" {
				v.Stat = "running"
				v.Speed = st.ist.speed
			}
			if st.op == "timeout" {
				v.Stat = "timeout"
			}
			return
		})

		v.l.Lock()
		if err != nil {
			v.Stat = "error"
			v.err = err
		} else {
			v.Stat = "done"
			v.save()
		}
		v.l.Unlock()
	}()

	return
}

func testVfile1(a []string) {
	vfileNewDownload("http://v.youku.com/v_show/id_XNTYxMjA4MTM2_ev_5.html", "/tmp/1")
	for {
		time.Sleep(time.Second)
	}
}

func testVfile2(a []string) {
	/*
	man := newVfileV2()

	for {
		man.l.Lock()
		running := 0
		errorn := 0
		hasinfoN := 0
		for _, v := range man.m {
			if v.Stat == "running" {
				running++
			}
			if v.Stat == "error" {
				errorn++
			}
			if v.Info.W != 0 {
				hasinfoN++
			}
		}
		man.l.Unlock()
		log.Printf("running %d error %d hasinfo %d", running, errorn, hasinfoN)
		time.Sleep(time.Second)
	}
	*/
}

