
package main

import (
	"github.com/hoisie/mustache"

	"bytes"
	"net/http"
	"crypto/sha1"
	"path/filepath"
	"time"
	"fmt"
	"log"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"encoding/json"
	"strings"
)

func test1() {
	avprobe("/work/1/a.mp4")
}

func avprobe(path string) (err error, dur float32, w,h int) {
	out, err := exec.Command("ffprobe", path).CombinedOutput()
	if err != nil {
		return
	}
	for _, l := range strings.Split(string(out), "\n") {
		var re *regexp.Regexp
		var ma []string
		re, _ = regexp.Compile(`Duration: (.{11})`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			var h,m,s,ms int
			fmt.Sscanf(ma[1], "%d:%d:%d.%d", &h, &m, &s, &ms)
			dur += float32(h)*3600
			dur += float32(m)*60
			dur += float32(s)
			dur += float32(ms)/100
			log.Printf("dur %v => %f", ma[1], dur)
		}
		re, _ = regexp.Compile(`Video: .* (\d+x\d+)`)
		ma = re.FindStringSubmatch(l)
		if len(ma) > 1 {
			fmt.Sscanf(ma[1], "%dx%d", &w, &h)
			log.Printf("wh %v => %dx%d", ma[1], w, h)
		}
	}
	return
}

func curl(url string) (body string, err error){
	var resp *http.Response
	resp, err = http.Get(url)
	if err != nil {
		return
	}
	var bodydata []byte
	bodydata, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	body = string(bodydata)
	return
}

func curldata(url string) (body []byte, err error) {
	var resp *http.Response
	resp, err = http.Get(url)
	if err != nil {
		return
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	return
}

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

func durstr(d float32) string {
	if d < 60*60 {
		return fmt.Sprintf("%d:%.2d", int(d/60), int(d)%60)
	}
	return fmt.Sprintf("%d:%.2d:%.2d", int(d/3600), int(d/60)%60, int(d)%60)
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

func splitContent(c string) (r []string) {
	for _, l := range strings.Split(c, "\n") {
		r = append(r, strings.Trim(l, "\r\n"))
	}
	return
}

func pathup (path string) string {
	arr := strings.Split(path, "/")
	if len(arr) <= 1 {
		return ""
	}
	return strings.Join(arr[0:len(arr)-1], "/")
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

func renderIndex(w io.Writer, body string) {
	s := mustache.RenderFile("tpl/index.html", map[string]string{"body": body})
	fmt.Fprintf(w, "%s", s)
}

func main() {

	global.menu = &menu{Flag:"dir"}
	global.menu.readFile("global")
	global.vfile = loadVfilemap()
	global.user = loadUsermap()

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

		renderIndex(w,
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
		for i, t := range types {
			if t.Value == m.Type {
				types[i].Checked = "checked"
			}
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

		renderIndex(w,
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

	vfilepage := func (w http.ResponseWriter, r *http.Request, path string) {
		v := global.vfile.shotsha(getsha1(path))
		if v != nil {
			http.Redirect(w, r, fmt.Sprintf("/vfile/%s", getsha1(path)), 302)
			return
		}
		v = global.vfile.shotsha(path)
		if v == nil {
			return
		}

		renderIndex(w,
			mustache.RenderFile("tpl/viewVfile.html", map[string]interface{} {
				"url": v.Url,
				"statstr": v.Statstr(),
				"path": path,
				"starttm": v.Starttm,
				"isDownloading": v.Stat == "downloading",
				"isUploading": v.Stat == "uploading",
				"isError": v.Stat == "error",
				"progress": fmt.Sprintf("%.1f%%", v.progress*100),
				"speed": fmt.Sprintf("%s/s", sizestr(v.speed)),
				"tsTotal": len(v.Ts),
				"tsDown": v.downN,
				"error": fmt.Sprintf("%v", v.err),
			}))
	}

	manvfile := func (w io.Writer, path string) {
		list := global.vfile.shotall()
		s := listvfile(list)
		renderIndex(w, s)
	}

	vfileUpload := func (w io.Writer, path string) {
		renderIndex(w, mustache.RenderFile("tpl/vfileUpload.html", map[string]interface{}{}))
	}

	vfileM3u8 := func (w http.ResponseWriter, wr io.Writer, sha string, host string) {
		log.Printf("vfilem3u8: %s", sha)
		v := global.vfile.shotsha(sha)
		if v == nil {
			http.Error(w, "not found", 404)
			return
		}
		v.genM3u8(wr, host)
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
		live := r.FormValue("live")
		liveend := r.FormValue("liveend")
		switch {
		case at != "":
			fat := float32(0)
			fmt.Sscanf(at, "%f", &fat)
			list.genM3u8(w, host, "at", fat)
		case live != "":
			fat := tmdur2float(time.Since(m.tmstart))
			list.genLiveM3u8(w, host, fat)
		case liveend != "":
			fat := tmdur2float(time.Since(m.tmstart))
			list.genLiveEndM3u8(w, host, fat)
		default:
			list.genM3u8(w, host)
		}
	}

	sampleM3u8Starttm := time.Now()

	sampleM3u8 := func (r *http.Request, w io.Writer, host,path string) {
		log.Printf("samplem3u8 %s", path)
		list := global.vfile.shotall()
		list2 := vfilelist{}
		for _, v := range list.m {
			v.Ts = v.Ts[0:1]
			list2.m = append(list2.m, v)
			list2.dur += v.Ts[0].Dur
		}
		at := tmdur2float(time.Since(sampleM3u8Starttm))
		list2.genLiveM3u8(w, host, at)
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
		case ".ts", ".html", ".css", ".js":
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
			vfileM3u8(w, &bs, pathsplit(path, 2), r.Host)
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
			manvfile(w, pathsplit(path, 1))
		case strings.HasPrefix(path, "/vfile"):
			vfilepage(w, r, pathsplit(path, 1))
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

