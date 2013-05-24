
package main

import (
	"github.com/hoisie/mustache"

	"bytes"
	"net/http"
	"crypto/rand"
	"crypto/sha1"
	"path/filepath"
	"time"
	"fmt"
	"log"
	"io"
	"io/ioutil"
	"os"
	"encoding/json"
	"strings"
)

func getloopat(at, dur float32) float32 {
	n := int(at/dur)
	return at - float32(n)*dur
}

type globalS struct {
	menu *menu
	vfile vfilemap
	user usermap
}

var (
	global globalS
)

func trimContent(c string) (nc string) {
	nc = ""
	for _, s := range strings.Split(c, "\n") {
		nc += strings.Trim(s, " \t") + "\n"
	}
	return
}

func tmdur2float(dur time.Duration) float32 {
	a := float32(dur)/float32(time.Second)
	return a
}

func perstr(per float64) string {
	return fmt.Sprintf("%.1f%%", per)
}

func durstr(d float32) string {
	if d < 60*60 {
		return fmt.Sprintf("%d:%.2d", int(d/60), int(d)%60)
	}
	return fmt.Sprintf("%d:%.2d:%.2d", int(d/3600), int(d/60)%60, int(d)%60)
}

func tmdurstr(d time.Duration) string {
	return durstr(tmdur2float(d))
}


func sizestr(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(size)/1024/1024)
	}
	return fmt.Sprintf("%.1fG", float64(size)/1024/1024/1024)
}

func getsha1(url string) string {
	h := sha1.New()
	io.WriteString(h, url)
	s := fmt.Sprintf("%x", h.Sum(nil))[:7]
	return s
}

func pathup (path string) string {
	arr := strings.Split(path, "/")
	if len(arr) <= 1 {
		return ""
	}
	return strings.Join(arr[0:len(arr)-1], "/")
}

func pathidx(path string, idx int) string {
	a := pathSplit(path)
	if idx < len(a) {
		return a[idx]
	}
	return ""
}

func pathsub(path string, start int) string {
	a := pathSplit(path)
	if len(a) <= start {
		return ""
	} else {
		return strings.Join(a[start:], "/")
	}
}

func pathsplit(path string, from int) string {
	a := filepath.Clean(strings.Trim(path, "/"))
	b := strings.Split(a, "/")
	if len(b) <= from {
		return ""
	} else {
		return strings.Join(b[from:], "/")
	}
}

func renderIndex(w io.Writer, sel,body string) {
	mp :=	map[string]string{
		"body": body,
	}
	mp[sel+"Sel"] = "active"
	s := mustache.RenderFile("tpl/index.html", mp)
	fmt.Fprintf(w, "%s", s)
}

func pathSplit(path string) []string {
	path = filepath.Clean(path)
	path = strings.Trim(path, "/")
	return strings.Split(path, "/")
}

func randsha1() (string) {
	h := sha1.New()
	io.CopyN(h, rand.Reader, 10)
	return fmt.Sprintf("%x", h.Sum(nil))[:7]
}

func loadJson(path string, v interface{}) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, v)
}

func saveJson(path string, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	ioutil.WriteFile(path, data, 0777)
}

func main() {

	log.Printf("argv %v", os.Args)

	testfuncs := []struct {
		name string
		cb func([]string)
	}{
		{"curl3", testCurl3},
		{"downv", testDownVfile},
		{"v1", testVfile1},
		{"v2", testVfile2},
		{"testhttp", testhttp},
		{"testajax", testajax},
		{"proxy1", testproxy1},
	}
	for _, f := range testfuncs {
		if len(os.Args) >= 2 && os.Args[1] == f.name {
			log.Printf("calling %s", f.name)
			f.cb(os.Args[1:])
			return
		}
	}

	//testMenuV2()
	//return

	if len(os.Args) >= 2 && os.Args[1] == "testavconv" {
		testavconv()
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "testcmd" {
		testcmd()
		return
	}

	testu := false

	if len(os.Args) >= 2 && os.Args[1] == "testu" {
		testu = true
	}

	global.menu = loadMenu()
	global.vfile = loadVfilemap()
	global.user = loadUsermap(testu)

	if len(os.Args) >= 2 && os.Args[1] == "testv" {
		testvfile()
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "test1" {
		test1()
		return
	}

	editdirPage := func (w io.Writer, path string, op string) {
		m := global.menu.get(path, nil)
		title := ""
		desc := ""
		if op == "edit" {
			if m == nil {
				return
			}
			title = "修改目录"
			desc = m.Desc
		} else {
			title = "添加目录"
		}

		renderIndex(w, "menu",
			mustache.RenderFile("tpl/editDir.html", map[string]interface{} {
				"title": title,
				"path": path2title(path),
				"action": fmt.Sprintf("/do_%sdir/%s", op, path),
				"desc": desc,
				"backurl": fmt.Sprintf("/menu/%s", path),
			}))
	}

	editvidPage := func (w io.Writer, path string, op string) {
		m := global.menu.get(path, nil)
		content := ""
		title := ""
		desc := ""
		types := []*struct {
			Value,Checked,Desc string
		}{
			{Value:"live", Desc:"直播"},
			{Value:"ondemand", Desc:"点播"},
		}
		checked := false
		for i, t := range types {
			if t.Value == m.Type {
				types[i].Checked = "checked"
				checked = true
			}
		}
		if !checked {
			types[0].Checked = "checked"
		}

		if op == "edit" {
			if m == nil {
				return
			}
			title = "修改视频"
			content = m.Content
			desc = m.Desc
		} else {
			title = "添加视频"
		}

		renderIndex(w, "menu",
			mustache.RenderFile("tpl/editVid.html", map[string]interface{} {
				"title": title,
				"path": path2title(path),
				"action": fmt.Sprintf("/do_%svid/%s", op, path),
				"desc": desc,
				"content": content,
				"backurl": fmt.Sprintf("/menu/%s", path),
				"types": types,
			}))
	}

	doUpload := func (r *http.Request, path string) {
		log.Printf("upload: newfile %s", path)

		mr, err := r.MultipartReader()
		if err != nil {
			return
		}
		length := r.ContentLength
		part, err := mr.NextPart()
		if err != nil {
			return
		}

		filename := part.FileName()
		ext := filepath.Ext(filename)
		log.Printf("upload: newfile filename %s ext %s", filename, ext)
		global.vfile.upload(path, ext, part, length)
		log.Printf("upload: end")
	}

	cgipage := func (w http.ResponseWriter, r *http.Request, path string) {
		log.Printf("cgi: path %s", path)
		if strings.HasPrefix(path, "upload") {
			doUpload(r, pathsplit(path, 1))
			return
		}
		switch r.FormValue("do") {
		case "downAllVfile":
			m := global.menu.get(path, nil)
			if m == nil || m.Flag != "url" {
				return
			}
			for _, line := range splitContent(m.Content) {
				if strings.HasPrefix(line, "http") {
					global.vfile.download(line)
				}
			}
			http.Redirect(w, r, fmt.Sprintf("/menu/%s", path), 302)
		case "downvfile":
			url := r.FormValue("url")
			v := global.vfile.download(url)
			http.Redirect(w, r, fmt.Sprintf("/vfile/%s", v.sha), 302)
		case "userinterim":
			global.user.interim(r)

		case "editVfilePage":
			doEditVfilePage(w, r)
		}
	}

	doedit := func (w http.ResponseWriter, r *http.Request, path string, op string, flag string) {
		if r.FormValue("desc") == "" {
			fmt.Fprintf(w, `<p>标题不能为空 [<a href="/menu/%s">返回</a>]</p>`, path)
			return
		}
		var m *menu
		if op == "add" {
			if flag == "url" {
				m = global.menu.addUrl(path, "", "", "", r.FormValue("type"))
			} else {
				m = global.menu.addDir(path, "", "")
			}
		} else {
			m = global.menu.get(path, nil)
		}
		if m == nil {
			return
		}
		m.Desc = r.FormValue("desc")
		m.Content = r.FormValue("content")
		m.Type = r.FormValue("type")
		global.menu.writeFile("global")

		if true {
			http.Redirect(w, r, fmt.Sprintf("/menu/%s", path), 302)
		} else {
			fmt.Fprintf(w, `<p>修改成功 [<a href="/menu/%s">返回</a>]</p>`, path)
		}
	}

	dodel := func (w io.Writer, path string) {
		global.menu.del(path)
		fmt.Fprintf(w, `<p>删除成功 [<a href="/menu/%s">返回</a>]</p>`, pathup(path))
		global.menu.writeFile("global")
	}

	trydel := func (w io.Writer, path string) {
		m := global.menu.get(path, nil)
		if m == nil {
			return
		}
		fmt.Fprintf(w, "<p>确认删除 '" + path2title(path) + "' ?</p>")
		fmt.Fprintf(w, `<a href="/do_del/%s">确定</a> | `, path)
		fmt.Fprintf(w, `<a href="/menu/%s">返回</a>`, path)
	}

	vfileM3u8 := func (r *http.Request, w http.ResponseWriter, wr io.Writer, sha string, host string) {
		log.Printf("vfilem3u8: %s", sha)
		v := global.vfile.shotsha(sha)
		if v == nil {
			http.Error(w, "not found", 404)
			return
		}
		at := r.FormValue("at")
		switch {
		case at != "":
			fat := float32(0)
			fmt.Sscanf(at, "%f", &fat)
			v.genLiveEndM3u8(wr, host, fat)
		default:
			v.genM3u8(wr, host)
		}
	}

	menuM3u8 := func (r *http.Request, w http.ResponseWriter, wr io.Writer, path,host string) {
		log.Printf("menum3u8 %s", path)
		m := global.menu.get(path, nil)
		if m == nil || m.Flag != "url" {
			http.Error(w, "not found", 404)
			return
		}
		list := vfilelistFromContent(m.Content)
		at := r.FormValue("at")
		liveend := r.FormValue("liveend")
		switch {
		case at != "":
			fat := float32(0)
			fmt.Sscanf(at, "%f", &fat)
			list.genLiveEndM3u8(w, host, fat)
		case liveend != "":
			fat := tmdur2float(time.Since(m.tmstart))
			list.genLiveEndM3u8(w, host, fat)
		default:
			list.genM3u8(w, host)
		}
	}

	playM3u8 := func (w io.Writer, url string) {
		log.Printf("playm3u8 %s", url)
		fmt.Fprintf(w, "<html>")
		fmt.Fprintf(w, "<body>")
		fmt.Fprintf(w, `<video src="%s" autoplay></video>`, url)
		fmt.Fprintf(w, "</body>")
		fmt.Fprintf(w, "</html>")
	}

	jsonMenu := func (w io.Writer, host,path string) {
		m := global.menu.get(path, nil)
		if m == nil {
			return
		}
		log.Printf("json menu: %s", path)
		m.fillM3u8Url(host, path)
		enc := json.NewEncoder(w)
		enc.Encode(m)
	}

	jsonCallback := func (w io.Writer, host,path string) {
		type S struct {
			Type string
			V interface{}
		}
		m := global.menu.get(path, nil)
		if m == nil {
			return
		}
		m.fillM3u8Url(host, path)
		s := S{"menu", m}

		enc := json.NewEncoder(w)
		enc.Encode(s)
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)
		log.Printf("GET %s", path)
		dir, file := filepath.Split(path)
		ext := filepath.Ext(file)
		switch ext {
		case ".ts", ".html", ".css", ".js", ".mp4", ".rm", ".rmvb", ".avi", ".mkv":
			http.ServeFile(w, r, path[1:])
		case ".m3u8":
			path = dir
		}

		var bs bytes.Buffer
		oneshot := func () {
			w.Header().Add("Content-Length", fmt.Sprintf("%d", (bs.Len())))
			w.Write(bs.Bytes())
		}

		switch {
		case path == "/":
			http.Redirect(w, r, "/menu", 302)

		case strings.HasPrefix(path, "/menu"):
			menuPage(w, pathsplit(path, 1))

		case strings.HasPrefix(path, "/users"):
			usersPage(w, pathsplit(path, 1))
		case strings.HasPrefix(path, "/user"):
			userPage(w, pathsplit(path, 1))

		case strings.HasPrefix(path, "/m3u8/vfile"):
			vfileM3u8(r, w, &bs, pathsplit(path, 2), r.Host)
			oneshot()
		case strings.HasPrefix(path, "/m3u8/menu"):
			menuM3u8(r, w, &bs, pathsplit(path, 2), r.Host)
			oneshot()
		case strings.HasPrefix(path, "/m3u8/sample"):
			sampleM3u8(r, &bs, r.Host, pathsplit(path, 2))
			oneshot()

		case strings.HasPrefix(path, "/players"):
			menuPlayersPage(w, pathsplit(path, 1))

		case strings.HasPrefix(path, "/json/menu"):
			jsonMenu(w, r.Host, pathsplit(path, 2))
		case strings.HasPrefix(path, "/json/callback"):
			jsonCallback(w, r.Host, pathsplit(path, 2))

		case strings.HasPrefix(path, "/play"):
			playM3u8(w, "/m3u8/"+pathsplit(path,1)+"/a.m3u8?"+r.URL.RawQuery)
		case strings.HasPrefix(path, "/addvid"):
			editvidPage(w, pathsplit(path, 1), "add")
		case strings.HasPrefix(path, "/adddir"):
			editdirPage(w, pathsplit(path, 1), "add")
		case strings.HasPrefix(path, "/editvid"):
			editvidPage(w, pathsplit(path, 1), "edit")
		case strings.HasPrefix(path, "/editdir"):
			editdirPage(w, pathsplit(path, 1), "edit")
		case strings.HasPrefix(path, "/do_editdir"):
			doedit(w, r, pathsplit(path, 1), "edit", "dir")
		case strings.HasPrefix(path, "/do_adddir"):
			doedit(w, r, pathsplit(path, 1), "add", "dir")
		case strings.HasPrefix(path, "/do_editvid"):
			doedit(w, r, pathsplit(path, 1), "edit", "url")
		case strings.HasPrefix(path, "/do_addvid"):
			doedit(w, r, pathsplit(path, 1), "add", "url")
		case strings.HasPrefix(path, "/del"):
			trydel(w, pathsplit(path, 1))
		case strings.HasPrefix(path, "/do_del"):
			dodel(w, pathsplit(path, 1))
		case strings.HasPrefix(path, "/manv/upload"):
			vfileUpload(w, pathsplit(path, 2))
		case strings.HasPrefix(path, "/manv"):
			manvfilePage(w, pathsplit(path, 1))

		case strings.HasPrefix(path, "/edit/vfile"):
			editVfilePage(w, r, pathsplit(path, 2))
		case strings.HasPrefix(path, "/vfile"):
			vfilePage(w, r, pathsplit(path, 1))

		case strings.HasPrefix(path, "/cgi"):
			cgipage(w, r, pathsplit(path, 1))
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	})
	err := http.ListenAndServe(":9191", nil)
	if err != nil {
		log.Printf("%v", err)
	}
}

