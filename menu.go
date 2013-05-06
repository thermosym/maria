
package main

import (
	"github.com/hoisie/mustache"

	"time"
	"io"
	"encoding/json"
	"strings"
	"fmt"
	"io/ioutil"
	"log"
)

type menu struct {
	Desc string
	Flag string
	Type string
	Content string
	M3u8Url string
	Sub map[string]*menu

	tmstart time.Time
}

func (m *menu) dump(w io.Writer) {
	enc := json.NewEncoder(w)
	enc.Encode(m)
}

func (m *menu) dumptree(indent int) {
	is := ""
	for i := 0; i < indent; i++ {
		is += " "
	}
	for k, s := range m.Sub {
		log.Printf("%s%s\n", is, k)
		s.dumptree(indent+1)
	}
}

func (m *menu) load(r io.Reader) {
	dec := json.NewDecoder(r)
	dec.Decode(m)
}

func (m *menu) fillM3u8Url(host,path string) {
	m.M3u8Url = "http://"+host+"/m3u8/menu"+path+"/a.m3u8"
	if m.Type == "live" {
		m.M3u8Url += "?live=1"
	}
	log.Printf("fillm3u8 %s", path)
	for s, mc := range m.Sub {
		mc.fillM3u8Url(host, path+"/"+s)
	}
}

func (m *menu) get(path string, cb func(r,p *menu, id string)) (r *menu) {
	arr := strings.Split(strings.Trim(path, "/"), "/")
	r = m
	//log.Printf("menu get : %v", arr[0])
	if arr[0] == "" || arr[0] == "/" {
		return
	}
	for _, s := range arr {
		p := r
		r = r.Sub[s]
		if r == nil {
			return nil
		}
		if cb != nil {
			cb(r, p, s)
		}
	}
	return
}

func (m *menu) ls(path string) (r map[string]*menu) {
	p := m.get(path, nil)
	if p != nil {
		r = p.Sub
	}
	return
}

func (m *menu) statstr() string {
	return ""
}

func (m *menu) newid(a map[string]*menu) string {
	for i := 0; ; i++ {
		got := false
		id := fmt.Sprintf("%d", i)
		for k, _ := range a {
			if k == id {
				got = true
				break
			}
		}
		if !got {
			return id
		}
	}
	return ""
}

func (m *menu) addEntry(path, node, desc, content, flag, Type string) (r *menu) {
	p := m.get(path, nil)
	if p == nil {
		return
	}
	if p.Flag != "dir" {
		return
	}
	if node == "" {
		node = m.newid(p.Sub)
	}
	r = p.Sub[node]
	if r == nil {
		r = &menu{Sub:map[string]*menu{}}
		p.Sub[node] = r
	}
	r.Desc = desc
	r.Flag = flag
	r.Content = trimContent(content)
	return r
}

func (m *menu) del(path string) {
	var lastp *menu
	var lastid string
	ret := m.get(path, func (r,p *menu, id string) {
		lastp = p
		lastid = id
	})
	if ret == nil {
		return
	}
	if lastp == nil {
		m.Sub = map[string]*menu{}
	} else {
		delete(lastp.Sub, lastid)
	}
	log.Printf("del %s", path)
	log.Printf(" lastp %v %s", lastp, lastid)
	m.dumptree(0)
}

func (m *menu) addDir(path, node, desc string) *menu {
	return m.addEntry(path, node, desc, "", "dir", "")
}

func (m *menu) addUrl(path, node, desc, content, Type string) *menu {
	return m.addEntry(path, node, desc, content, "url", Type)
}

func (m *menu) writeFile(filename string) {
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("%v", err)
		return
	}
	ioutil.WriteFile(filename, data, 0777)
}

func (m *menu) readFile(filename string) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Printf("%v", err)
		return
	}
	json.Unmarshal(data, m)

	m.foreach(func (r *menu) {
		if r.Type == "live" {
			r.tmstart = time.Now()
		}
	})
}

func (m *menu) foreach(cb func (r *menu)) {
	cb(m)
	for _, s := range m.Sub {
		s.foreach(cb)
	}
}
func testMenu() {
	m := &menu{Flag:"dir", Sub:map[string]*menu{}}
	m.addDir("/", "youku", "优酷视频")
	m.addDir("/", "sohu", "搜狐视频")
	m.addUrl("/youku", "movie", "优酷电影", `
		http://v.youku.com/v_show/id_XNTQzMDc1OTgw.html?f=18898121
		http://v.youku.com/v_show/id_XNTM4NDU1NTA4.html
		http://v.youku.com/v_show/id_XNTM2NDA4Nzg4.html
		http://v.youku.com/v_show/id_XNTQwOTUxODg4.html
		http://v.youku.com/v_show/id_XNTQ0MDEzMjY0.html
		http://v.youku.com/v_show/id_XNTQwMTMzODMy.html
	`, "live")
	m.addUrl("/youku", "series", "优酷电视剧", `
		http://v.youku.com/v_show/id_XNTQzNjEwMDg4.html
		http://v.youku.com/v_show/id_XNTM3NjYzNDcy.html
		http://v.youku.com/v_show/id_XNDY2NjM0MjEy.html
		http://v.youku.com/v_show/id_XNDg0NzE2NTQw.html
		http://v.youku.com/v_show/id_XNTE3MDIzMTM2.html
		http://v.youku.com/v_show/id_XNTMyODc2NzYw.html
		http://v.youku.com/v_show/id_XNTM1OTkyMTQ4.html
	`, "live")
	m.addUrl("/youku", "yule", "优酷娱乐", `
		http://v.youku.com/v_show/id_XNTM4NDU3MzEy.html
		http://v.youku.com/v_show/id_XMzUwMDU2Nzky.html
		http://v.youku.com/v_show/id_XNTIxOTcyOTIw.html
		http://v.youku.com/v_show/id_XNDgzNDU0MzIw.html
		http://v.youku.com/v_show/id_XNDg3MDIwMzI4.html
		http://v.youku.com/v_show/id_XNDc1OTEwMzQw.html
	`, "live")
	m.addUrl("/youku", "news", "优酷新闻", `
		http://v.youku.com/v_show/id_XNTQzOTkwNzEy.html?f=19173059
		http://v.youku.com/v_show/id_XNTQ0MDAxMjE2.html?f=19175120
		http://v.youku.com/v_show/id_XNTQzOTg5MTI0.html?f=19173166
		http://v.youku.com/v_show/id_XNTQzNzM3NTM2.html?f=19176152
		http://v.youku.com/v_show/id_XNTQzOTk5NzQ0.html?f=19173166
	`, "live")
	m.addUrl("/sohu", "news", "搜狐新闻", `
		http://tv.sohu.com/20130417/n372980882.shtml
		http://tv.sohu.com/20130416/n372836760.shtml/index.shtml/index.shtml
		http://tv.sohu.com/20130415/n372722202.shtml/index.shtml/index.shtml
		http://tv.sohu.com/20130417/n372966923.shtml
		http://tv.sohu.com/20130415/n372751256.shtml/index.shtml/index.shtml
	`, "live")
	m.addUrl("/sohu", "yule", "搜狐娱乐", `
		http://tv.sohu.com/20130416/n372914676.shtml
		http://tv.sohu.com/20130416/n372774957.shtml
		http://tv.sohu.com/20130415/n372768491.shtml
		http://tv.sohu.com/20130129/n364976523.shtml
		http://tv.sohu.com/20130416/n372901760.shtml
	`, "live")
	m.addUrl("/sohu", "series", "搜狐电视剧", `
		http://tv.sohu.com/20130411/n372388445.shtml
		http://tv.sohu.com/20130416/n372907125.shtml
		http://tv.sohu.com/20120915/n353219021.shtml
		http://tv.sohu.com/20121212/n360255747.shtml
		http://tv.sohu.com/20130405/n371760482.shtml
	`, "live")
	m.addUrl("/sohu", "movie", "搜狐电影", `
		http://tv.sohu.com/20130417/n372981909.shtml
		http://tv.sohu.com/20130409/n372077553.shtml
		http://tv.sohu.com/20130407/n371829027.shtml
		http://tv.sohu.com/20130408/n371935984.shtml
		http://tv.sohu.com/20130320/n369601988.shtml
	`, "live")

	m.dumptree(0)
	global.menu = m
	m.writeFile("global")
}


func menuPage(w io.Writer, path string) {
	m := global.menu.get(path, nil)
	if m == nil {
		return
	}

	title := "编辑菜单: " + path2title(path)
	titles := path2titles(path)
	titlelast := ""
	if len(titles) > 0 {
		n := len(titles)
		titlelast = titles[n-1].Desc
		titles = titles[0:n-1]
	}

	type btn struct {
		Href, Title string
	}
	btns := []btn{}
	btns2 := []btn{}

	if path != "" {
		btns = append(btns, btn{"/menu/"+pathup(path), "上一级目录"})
	}
	if m.Flag == "dir" {
		btns = append(btns, btn{"/adddir/"+path, "添加目录"})
		btns = append(btns, btn{"/addvid/"+path, "添加视频"})
		if path != "" {
			btns = append(btns, btn{"/editdir/"+path, "修改"})
		}
	}
	btns = append(btns, btn{"/del/"+path, "删除"})

	type menuH struct {
		Tstr,Path,Desc string
	}
	mharr := []menuH{}
	liststr := ""

	at := float32(0)
	elapsed := float32(0)

	if m.Flag == "dir" {
		marr := global.menu.ls(path)
		for k, s := range marr {
			var tstr string
			switch s.Flag {
			case "dir":
				tstr = "目录"
			case "url":
				tstr = "视频"
			}
			switch s.Type {
			case "live":
				tstr = "直播"
			case "ondemand":
				tstr = "点播"
			}
			mharr = append(mharr, menuH{tstr,"/menu/"+path+"/"+k, s.Desc})
		}
	} else {
		btns2 = append(btns2, btn{"/editvid/"+path, "编辑"})
		btns2 = append(btns2, btn{"/m3u8/menu/"+path, "查看m3u8"})
		btns2 = append(btns2, btn{"/play/menu/"+path, "播放m3u8"})
		btns2 = append(btns2, btn{"/cgi/"+path+"/?do=downAllVfile", "下载全部"})

		var list *vfilelist

		if m.Content != "" {
			list = vfilelistFromContent(m.Content)
		}

		if list == nil || len(list.m) == 0 {
			liststr = `<p>[空]</p>`
		} else {
			liststr = listvfile(list)
		}

		if m.Type == "live" && list != nil && list.dur > 0 {
			elapsed = tmdur2float(time.Since(m.tmstart))
			at = elapsed - list.dur*float32(int(elapsed/list.dur))
		}
	}

	renderIndex(w,
	mustache.RenderFile("tpl/menuPage.html", map[string]interface{} {
		"btns": btns,
		"btns2": btns2,
		"isDir": m.Flag == "dir",
		"listEmpty": len(mharr) == 0,
		"list": mharr,
		"liststr": liststr,
		"title": title,
		"titles": titles,
		"titlelast": titlelast,
		"isLive": m.Type == "live",
		"tmelapsed": durstr(elapsed),
		"tmat": durstr(at),
	}))
}

type menuTitleS struct {
	Desc,Href string
}

func path2titles (path string) (tarr []menuTitleS) {
	tarr = append(tarr, menuTitleS{"主菜单", ""})
	global.menu.get(path, func (r,p *menu, id string) {
		tarr = append(tarr, menuTitleS{r.Desc, ""})
	})
	return tarr
}

func path2title (path string) string {
	tarr := []string{}
	global.menu.get(path, func (r,p *menu, id string) {
		tarr = append(tarr, r.Desc)
	})
	if len(tarr) == 0 {
		return "根目录"
	} else {
		return strings.Join(tarr, " / ")
	}
}

