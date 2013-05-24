
package main

import (
	"io"
	"os"
	"time"
	"strings"
	"fmt"
	"log"
	"errors"
	"sync"
	"path/filepath"
)

type vfileV2Node struct {
	Stat string
	Name string
	Type string
	Src string
	Desc string
	Size int64
	Filename string
	Dur time.Duration
	Ts []tsinfo2
	Starttm time.Time
	Info avprobeStat

	name string
	path string

	l *sync.Mutex

	forceStop bool
	downSt downloadStat
	upSt iocopyStat
	convSt avconvStat
	vproxySt vproxyStat
	err error
}

func (m *vfileV2Node) log(format string, v ...interface{}) {
	str := fmt.Sprintf(format, v...)
	log.Printf("vfile %s: %s", m.name, str)
}

func loadVfileV2Node(path string, name string) (v *vfileV2Node) {
	v = &vfileV2Node{}
	v.path = path
	v.name = name
	v.l = &sync.Mutex{}
	loadJson(filepath.Join(path, "info"), v)
	return
}

func (m *vfileV2Node) save() {
	m.l.Lock()
	defer m.l.Unlock()
	saveJson(filepath.Join(m.path, "info"), m)
}

func (m *vfileV2Node) rm() {
	m.l.Lock()
	defer m.l.Unlock()
	m.forceStop = true
	os.RemoveAll(m.path)
}

func (m *vfileV2Node) set(args form) {
	m.l.Lock()
	defer m.l.Unlock()
	t := args.str("type")
	switch t {
	case "youku","sohu","upload":
		m.Type = t
	}
	d := args.str("desc")
	if d != "" {
		m.Desc = d
	}
}

func (v vfileV2Node) Statstr() string {
	stat := ""
	switch v.Stat {
	case "parsing":
		stat += "[解析中]"
	case "downloading":
		stat += fmt.Sprintf("[下载中%.1f%%]", v.downSt.per*100)
		stat += fmt.Sprintf("[%s]", speedstr(v.downSt.speed))
		stat += fmt.Sprintf("[%s]", sizestr(v.Size))
	case "done":
		stat += "[已完成]"
		stat += fmt.Sprintf("[%s]", sizestr(v.Size))
	case "uploading":
		stat += fmt.Sprintf("[上传中%.1f%%]", v.upSt.per*100)
		stat += fmt.Sprintf("[%s]", sizestr(v.Size))
	case "avconving":
		stat += fmt.Sprintf("[转码中%.1f%%]", v.convSt.per*100)
		stat += fmt.Sprintf("[%s]", speedstr(v.convSt.speed))
		stat += fmt.Sprintf("[%s]", sizestr(v.Size))
	case "error":
		stat += "[出错]"
	case "nonexist":
		stat += "[不存在]"
	default:
		stat += "[不存在]"
	}
	if v.Dur > 0.0 {
		stat += fmt.Sprintf("[%s]", tmdurstr(v.Dur))
	}
	if v.Info.W != 0 {
		stat += fmt.Sprintf("[%dx%d]", v.Info.W, v.Info.H)
	}
	return stat
}

func uploadVfileConv(
	filename,path string,
	r io.Reader,
	length int64,
	cb ioCopyCb,
	cb2 func(),
	cb3 avconvCb,
	) (err error, size int64) {

	var w io.Writer
	w, err = os.Create(path)
	if err != nil {
		err = errors.New(fmt.Sprintf("create %s failed", path))
		return
	}

	err, _, size = ioCopy(r, length, w, cb)
	if err != nil {
		err = errors.New(fmt.Sprintf("upload %s failed: %s", filename, err))
		return
	}

	cb2()

	err = avconvM3u8V2(filename, path, cb3)
	if err != nil {
		err = errors.New(fmt.Sprintf("avconv %s failed: %s", filename, err))
	}

	return
}

func (v *vfileV2Node) upload(filename string, r io.Reader, length int64) {
	v.l.Lock()
	v.Stat = "uploading"
	v.Type = "upload"
	v.forceStop = false
	v.Filename = filename
	v.l.Unlock()

	iocopy := func (st iocopyStat) (err error) {
		v.l.Lock()
		defer v.l.Unlock()
		if v.forceStop {
			return errors.New("user force stop uploading")
		}
		v.upSt = st
		return
	}

	done := func() {
		v.l.Lock()
		defer v.l.Unlock()
		v.Stat = "avconving"
	}

	avconv := func (st avconvStat) (err error) {
		v.l.Lock()
		defer v.l.Unlock()
		if v.forceStop {
			return errors.New("user force stop avconv")
		}
		v.convSt = st
		return
	}

	err, size := uploadVfileConv(
		filepath.Join(v.path, filename), v.path, r, length,
		iocopy, done, avconv,
	)

	v.l.Lock()
	if err != nil {
		v.Stat = "error"
		v.err = err
	} else {
		v.Size = size
	}
	v.save()
	v.l.Unlock()
}

func (v *vfileV2Node) downloadCheck(url string) error {
	if !strings.Contains(url, "youku") && !strings.Contains(url, "sohu") {
		return errors.New("url invalid")
	}
	return nil
}

func (v *vfileV2Node) download(url string) {
	v.l.Lock()
	v.Stat = "parsing"
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
		v.downSt = st
		v.Size = st.size
		if st.stat == "firstTs" {
			err, v.Info = avprobe2(st.filename)
		}
		if st.stat == "parsedM3u8" {
			v.Stat = "downloading"
			v.Dur = st.dur
		}
		v.log("download %s %s", st.stat, perstr(st.per))
		return err
	})

	v.l.Lock()
	if err != nil {
		v.Stat = "error"
		v.err = err
		v.log("download error %s", err)
	} else {
		v.Stat = "done"
		v.Ts = v.downSt.ts
		v.log("download done")
		v.save()
	}
	v.l.Unlock()
}

func (v vfileV2Node) Typestr() string {
	switch v.Type {
	case "youku":
		return "优酷下载"
	case "sohu":
		return "搜狐下载"
	case "vproxy":
		return "在线直播"
	case "upload":
		return "用户上传"
	}
	return "未知类型"
}

func (v vfileV2Node) Geostr() string {
	if v.Info.W == 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", v.Info.W, v.Info.H)
}

func (v vfileV2Node) Sizestr() string {
	if v.Size == 0 {
		return ""
	}
	return sizestr(v.Size)
}

func testVfile1(_a []string) {
	m := loadVfileV2Node("/tmp/", "hehe")
	m.save()
}

func testVfile2(_a []string) {
	m := loadVfileV2Node("/tmp/", "hehe")
	go m.download("http://v.youku.com/v_show/id_XMzc2NDM4Nzc2.html")
	for {
		log.Printf("%v %v\n", m.Statstr(), m.err)
		time.Sleep(time.Second)
	}
}

