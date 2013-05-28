
package main

import (
	"strings"
	"encoding/json"
	"time"
	"net/http"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
	"bufio"
	"os"
	"errors"
	"fmt"
)

var (
	sampleM3u8Starttm = time.Now()
)

func readLines(path string, cb func(string) error) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		err = cb(strings.Trim(line, "\r\n"))
		if err != nil {
			break
		}
	}
	f.Close()
}

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

type form interface {
	strs2(key string) ([]string,bool)
	str2(key string) (string,bool)
	str(key string) string
	strs(key string) []string
}

type hash map[string]interface{}

func (m hash) str2(key string) (val string, ok bool) {
	var vals []string
	vals, ok = m.strs2(key)
	if ok {
		val = vals[0]
	}
	return
}

func (m hash) str(key string) (val string) {
	val, _ = m.str2(key)
	return
}

func (m hash) strs2(key string) (vals []string, ok bool) {
	var _vals interface{}
	_vals, ok = m[key]
	if ok {
		switch _vals.(type) {
		case []string:
			vals = _vals.([]string)
		case string:
			vals = []string{_vals.(string)}
		}
	}
	return
}

func (m hash) strs(key string) (vals []string) {
	vals, _ = m.strs2(key)
	return
}

type globalV2 struct {
	menu *menuV2
	vfile *vfileV2
	vlist *vlistV2
	user *userV2
}

var (
	vm globalV2
)

func loadVM() {
	vm.menu = loadMenuV2(".")
	vm.vfile = loadVfileV2(".")
	//vm.vfile = loadVfileFromCsv("tvlistxml/sample3")
	vm.vlist = loadVlistV2()
	vm.user = loadUserV2()
}

func jsonWrite(w io.Writer, a interface{}) {
	b, _ := json.Marshal(a)
	w.Write(b)
}

func parseForm(r *http.Request) (ret hash) {
	ret = hash{}
	_body, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(_body, &ret)
	r.ParseForm()
	for k,v := range r.Form {
		ret[k] = v
	}
	return
}

func testhttp(_a []string) {

	loadVM()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//path := filepath.Clean(r.URL.Path)
		path := r.URL.Path
		dir, file := filepath.Split(path)
		ext := filepath.Ext(file)

		form := parseForm(r)

		if path == "/" {
			http.ServeFile(w, r, "tpl/index.html")
			return
		}

		switch ext {
		case ".ts", ".html", ".css",".js", ".mp4",
				 ".rm", ".rmvb", ".avi", ".mkv", ".ico":
			http.ServeFile(w, r, filepath.Clean(path)[1:])
			return
		case ".m3u8":
			path = dir
		}

		var ret interface{}
		var err error

		log.Printf("%s %s %v", r.Method, path, form)

		switch path {
			case "/vfile":
				err, ret = vm.vfile.post(form)
			case "/menu":
				err, ret = vm.menu.post(form)
			case "/vlist":
				err, ret = vm.vlist.post(form)
			default:
				err = errors.New(fmt.Sprintf("wrong path %s", path))
		}

		if err != nil {
			ret = hash{"err":fmt.Sprintf("%s", err)}
		}

		b, _ := json.Marshal(ret)
		log.Printf("  %s", string(b))
		w.Write(b)
	})

	err := http.ListenAndServe(":9191", nil)
	if err != nil {
		log.Printf("%v", err)
	}
}


