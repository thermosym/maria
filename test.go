
package main

import (
	"encoding/json"
	"fmt"
	"bytes"
	"time"
	"net/http"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
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

type myform struct {
	r *http.Request
	body hash
}

func (m myform) str2(key string) (val string, ok bool) {
	var vals []string
	vals, ok = m.strs2(key)
	if len(vals) > 0 {
		val = vals[0]
	}
	return
}

func (m myform) str(key string) (val string) {
	val, _ = m.str2(key)
	return
}

func (m myform) strs2(key string) (vals []string, ok bool) {
	vals, ok = m.r.Form[key]
	if !ok {
		vals, ok = m.body.strs2(key)
	}
	return
}

func (m myform) strs(key string) (vals []string) {
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
	vm.vlist = loadVlistV2()
	vm.user = loadUserV2()
}

func jsonWrite(w io.Writer, a interface{}) {
	b, _ := json.Marshal(a)
	w.Write(b)
}

func jsonErr(w io.Writer, err error) {
	jsonWrite(w, hash{"err": fmt.Sprintf("%s", err)})
}

func testhttp(_a []string) {

	loadVM()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)
		log.Printf("%s %s", r.Method, path)
		dir, file := filepath.Split(path)
		ext := filepath.Ext(file)

		var form myform
		_body, _ := ioutil.ReadAll(r.Body)
		form.r = r
		json.Unmarshal(_body, &form.body)
		r.ParseForm()
		//log.Printf("data %s %v form %v", _body, form.body, r.Form)
		//log.Printf("testdata: err = %v %v", form.body.str("err"), form.body.strs("err"))
		log.Printf("  %v %v", r.Form, form.body)

		if path == "/" {
			http.ServeFile(w, r, "tpl/index.html")
			return
		}

		switch ext {
		case ".ts", ".html", ".css", ".js", ".mp4", ".rm", ".rmvb", ".avi", ".mkv":
			http.ServeFile(w, r, path[1:])
			return
		case ".m3u8":
			path = dir
		}

		mod := make([]string, 4)
		for i, _ := range mod {
			mod[i] = pathidx(path, i)
		}

		w2 := new(bytes.Buffer)

		if r.Method == "POST" {
			switch mod[0] {
			case "menu":
				vm.menu.post(mod[1], form, w2)
			case "vlist":
				vm.vlist.post(mod[1], form, w2)
			case "vfiles":
				vm.vfile.post(mod[1], form, w2)
			}
		} else {
			switch mod[0] {
			case "vfile":
				switch mod[1] {
				case "watch":
					jsonWrite(w2, vm.vfile.watch1(form))
				default:
					jsonWrite(w2, vm.vfile.one1(mod[1], form))
				}
			case "vfiles":
				switch mod[1] {
				case "list":
					jsonWrite(w2, vm.vfile.page1(form))
				}
			case "vlists":
				jsonWrite(w2, vm.vlist.page2(form))
			case "vlist":
				switch mod[1] {
				case "new":
					jsonWrite(w2, vm.vlist.new1(form))
				default:
					jsonWrite(w2, vm.vlist.page1(mod[1], form))
				}
			case "menu":
				jsonWrite(w2, vm.menu.view1(mod[1], form))
			case "users":
				jsonWrite(w2, vm.user.view1(mod[1]))
			}
		}
		str := string(w2.Bytes())
		log.Printf("%s", str)
		fmt.Fprintf(w, "%s", str)

	})
	err := http.ListenAndServe(":9191", nil)
	if err != nil {
		log.Printf("%v", err)
	}
}


