
package main

import (
	"github.com/hoisie/mustache"

	"encoding/json"
	"fmt"
	"time"
	"net/http"
	"io"
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
	str2(key string) (bool,string)
	str(key string) string
	strs(key string) []string
}

type hash map[string]interface{}

type myform struct {
	r *http.Request
}

func (m myform) str2(key string) (ok bool, val string) {
	var vals []string
	vals, ok = m.r.Form[key]
	if len(vals) > 0 {
		val = vals[0]
	}
	return
}

func (m myform) str(key string) (val string) {
	return m.r.FormValue(key)
}

func (m myform) strs(key string) (vals []string) {
	var ok bool
	vals, ok = m.r.Form[key]
	if !ok {
		vals, ok = m.r.URL.Query()[key]
	}
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

func jsonWrite(w io.Writer, a hash) {
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

		switch ext {
		case ".ts", ".html", ".css", ".js", ".mp4", ".rm", ".rmvb", ".avi", ".mkv":
			http.ServeFile(w, r, path[1:])
		case ".m3u8":
			path = dir
		}

		var test bool
		var mod string

		if pathidx(path, 0) == "test" {
			test = true
			path = pathsub(path, 1)
		}

		do := func (index bool, tpl string, a... interface{}) {
			str := mustache.RenderFile(tpl, a...)
			if test || index {
				renderIndex(w, mod, str)
			} else {
				fmt.Fprintf(w, "%s", str)
			}
		}

		mod = pathidx(path, 0)
		name1 := pathidx(path, 1)
		//name2 := pathidx(path, 2)
		form := myform{r}

		if r.Method == "POST" {
			switch mod {
			case "menu":
				vm.menu.post(path, r, w)
			case "vlist":
				vm.vlist.post(path, r, w)
			case "vfiles":
				vm.vfile.post(path, form, w)
			}
			return
		}

		switch mod {
		case "vfile":
			switch name1 {
			case "watch":
				do(false, "tpl/watch1.html", vm.vfile.watch1(form))
			}
		case "vfiles":
			switch name1 {
			case "list":
				do(false, "tpl/vlist1.html", vm.vfile.page1(form))
			default:
				do(true, "tpl/vfiles.html")
			}
		case "vlist":
			do(true, "tpl/vlist1.html", vm.vlist.page1(name1, form))
		case "menu":
			view1 := vm.menu.view1(name1, form)
			body2 := mustache.RenderFile("tpl/vlist1.html", view1.List)
			map1 := map[string]interface{}{"VList" : body2}
			body := mustache.RenderFile("tpl/menu1.html", view1, map1)
			renderIndex(w, "menu", body)
		case "users":
			view1 := vm.user.view1(name1)
			body := mustache.RenderFile("tpl/user1.html", view1)
			renderIndex(w, "users", body)
		}
	})
	err := http.ListenAndServe(":9191", nil)
	if err != nil {
		log.Printf("%v", err)
	}
}


