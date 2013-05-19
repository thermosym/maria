
package main

import (
	"net/http"
	"fmt"
	"log"
	"path/filepath"
	"github.com/hoisie/mustache"
	"strings"
	"math/rand"
)

func testajax(_a []string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)
		log.Printf("%s %s", r.Method, path)
		_, file := filepath.Split(path)
		ext := filepath.Ext(file)
		r.ParseForm()

		switch ext {
		case ".ts", ".html", ".css", ".js", ".mp4", ".rm", ".rmvb", ".avi", ".mkv":
			http.ServeFile(w, r, path[1:])
		}

		if path == "/dumpform" {
			fmt.Fprintf(w, "form = ")
			jsonWrite(w, r.Form)
			return
		}
		if path == "/seldel" {
			fmt.Fprintf(w, `<form>`)
			for i := 0; i < 4; i++ {
				fmt.Fprintf(w, `<input class="input" type="checkbox" value="%d" name="sel">%d</input>`, i, i)
			}
			fmt.Fprintf(w, `</form>`)
			fmt.Fprintf(w, `<a class="btn" do="ok form">Del</a>`)
		}
		if path == "/dumpform1" {
			fmt.Fprintf(w, "form = ")
			jsonWrite(w, r.Form)
			fmt.Fprintf(w, `<a class="btn" do="ok">Ok</a>`)
			fmt.Fprintf(w, `<a class="btn" do="cancel">Cancel</a>`)
			return
		}
		if path == "/err" {
			http.Error(w, "something happens", 404)
			return
		}
		if path == "/mod1" {
			fmt.Fprintf(w, `<p>module1</p>`)
			fmt.Fprintf(w, `<a class="btn" do="get $t 'p=banana&p=orange&p=apple'">Fruits</a>`)
			fmt.Fprintf(w, `<a class="btn" do="get $t 'p=alien&p=human&p=child'">Humans</a>`)
		}


		if r.Method == "POST" {
			switch path {
			case "/randstr":
				jsonWrite(w, hash{"list":rand.Intn(400)})
			}
			return
		}

		switch path {
		case "/":
			str := mustache.RenderFile("tpl/ajax.html")
			fmt.Fprintf(w, "%s", str)
		case "/showlist1":
			list := r.FormValue("list")
			if list != "" {
				for _, s := range strings.Split(list, ",") {
					fmt.Fprintf(w, `<a class="btn" href="#" listdel="%s">%s</a>`, s, s)
				}
			}
		}
	})
	err := http.ListenAndServe(":9192", nil)
	if err != nil {
		log.Printf("%v", err)
	}
}


