
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
	strs2(key string) ([]string,bool)
	str2(key string) (string,bool)
	str(key string) string
	strs(key string) []string
}

type hash map[string]interface{}

type myform struct {
	r *http.Request
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
		vals, ok = m.r.URL.Query()[key]
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
		r.ParseForm()
		log.Printf("%s %s %v", r.Method, path, r.Form)
		dir, file := filepath.Split(path)
		ext := filepath.Ext(file)

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

		var test bool
		if mod[0] == "test" {
			test = true
			mod = mod[1:]
		}

		do := func (index bool, tpl string, a ...interface{}) {
			a = append(a, hash{"test":test})
			str := mustache.RenderFile(tpl, a...)
			if test || index {
				renderIndex(w, mod[0], str)
			} else {
				fmt.Fprintf(w, "%s", str)
			}
		}

		if path == "/" || path == "/menu" {
			http.Redirect(w, r, "/menu/root", 302)
			return
		}

		form := myform{r}

		if r.Method == "POST" {
			switch mod[0] {
			case "menu":
				vm.menu.post(mod[1], form, w)
			case "vlist":
				vm.vlist.post(mod[1], form, w)
			case "vfiles":
				vm.vfile.post(mod[1], form, w)
			}
			return
		}

		switch mod[0] {
		case "vfile":
			switch mod[1] {
			case "watch":
				do(false, "tpl/watch1.html", vm.vfile.watch1(form))
			default:
				do(true, "tpl/vfile1.html", vm.vfile.one1(mod[1], form))
			}
		case "vfiles":
			switch mod[1] {
			case "list":
				do(false, "tpl/vlist1.html", vm.vfile.page1(form))
			default:
				do(true, "tpl/vfiles.html")
			}
		case "vlists":
			do(true, "tpl/vlist2.html", vm.vlist.page2(form))
		case "vlist":
			switch mod[1] {
			case "new":
				do(false, "tpl/vlist1.html", vm.vlist.new1(form))
			default:
				do(false, "tpl/vlist1.html", vm.vlist.page1(mod[1], form))
			}
		case "menu":
			do(true, "tpl/menu1.html", vm.menu.view1(mod[1], form))
		case "users":
			view1 := vm.user.view1(mod[1])
			body := mustache.RenderFile("tpl/user1.html", view1)
			renderIndex(w, "users", body)
		}
	})
	err := http.ListenAndServe(":9191", nil)
	if err != nil {
		log.Printf("%v", err)
	}
}


