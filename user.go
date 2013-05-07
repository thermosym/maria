
package main

import (
	"github.com/hoisie/mustache"

	"strings"
	"net/http"
	"log"
	"io"
	"fmt"
	"net/url"
	"path/filepath"
	"time"
)

type userHistoryS struct {
	watch,watchPath string
	dur float32
	time time.Time
}

type userHistory struct {
	m []*userHistoryS
	dur float32
}

type user struct {
	name string
	cpuinfo,sysinfo,meminfo string
	app,appver string
	watch,watchPath string

	history userHistory
}

type usermap struct {
	m map[string]*user
}

func loadUsermap(test bool) (m usermap) {
	m.m = map[string]*user{}
	if test {
		m.m["xieran"] = &user{
			name: "xieran",
			history: userHistory{
				m: []*userHistoryS{
					&userHistoryS{watch:"xx", watchPath:"m3u8/menu/youku/0", dur:33.0, time:time.Now()},
				},
				dur:33.0,
			},
		}
		return
	}
	return
}

func (m usermap) shotone(name string) (u *user) {
	var ok bool
	u, ok = m.m[name]
	if !ok {
		return nil
	}
	return
}

func (u *user) addHistory(watch,watchPath string, dur float32) {
	var h *userHistoryS
	if len(u.history.m) > 0 && u.history.m[len(u.history.m)-1].watch == watch {
		h = u.history.m[len(u.history.m)-1]
	} else {
		h = &userHistoryS{watch:watch, watchPath:watchPath, dur:dur, time:time.Now()}
		u.history.m = append(u.history.m, h)
	}
	h.dur += dur
	u.history.dur += dur
}

func (h *userHistoryS) TimeStr() string {
	return h.time.Format("15:04:05")
}

func (h *userHistoryS) DescHtml() string {
	return getWatchHtml(h.watch, h.watchPath)
}

func (h *userHistoryS) DurStr() string {
	return durstr(h.dur)
}

func getWatchPath(_url string) string {
	u, _ := url.Parse(_url)
	path := filepath.Dir(u.Path)
	path = strings.Trim(path, "/")
	return path
}

func (m usermap) interim(r *http.Request) {
	var name string
	name = r.FormValue("name")
	if name == "" {
		return
	}
	log.Printf("user %s: interim", name)
	var u *user
	var ok bool
	u, ok = m.m[name]
	if !ok {
		u = &user{}
		u.name = name
		m.m[name] = u
	}
	if r.FormValue("cpuinfo") != "" {
		u.cpuinfo = r.FormValue("cpuinfo")
	}
	if r.FormValue("meminfo") != "" {
		u.meminfo = r.FormValue("meminfo")
	}
	if r.FormValue("sysinfo") != "" {
		u.sysinfo = r.FormValue("sysinfo")
	}

	watch := r.FormValue("watch")
	watchPath := ""

	if watch != "" {
		watchPath = getWatchPath(watch)
		u.watch = watch
		u.watchPath = watchPath
		log.Printf("user %s: watch %s", name, u.watchPath)
	}
	if r.FormValue("app") != "" {
		u.app = r.FormValue("app")
	}
	if r.FormValue("appver") != "" {
		u.appver = r.FormValue("appver")
	}
	if r.FormValue("interval") != "" {
		var i float32
		fmt.Sscanf(r.FormValue("interval"), "%f", &i)
		if i > 0 && watch != "" {
			u.addHistory(watch,watchPath, i)
		}
	}
}

func (m usermap) shotall() (ret usermap) {
	return m
}

func userPage(w io.Writer, path string) {
	u := global.user.shotone(path)
	if u == nil {
		return
	}
	renderIndex(w, "user",
	mustache.RenderFile("tpl/userPage.html", map[string]interface{} {
		"name": u.name,
		"watch": fmt.Sprintf(`<a target=_blank href="%s">%s</a>`, u.watch, u.watch),
		"app": u.app,
		"appver": u.appver,
		"cpuinfo": u.cpuinfo,
		"meminfo": u.meminfo,
		"sysinfo": u.sysinfo,
		"hisDur": durstr(u.history.dur),
		"history": u.history.m,
	}))
}

type usersS struct {
	NameHref, Name string
	WatchHtml, Watch string
	Time string
}

func userlistPage(users []usersS) string {
	return mustache.RenderFile("tpl/usersPage.html", map[string]interface{} {
		"livenr":len(users),
		"users":users,
	})
}

func getWatchHtml(watch,watchPath string) (html string) {
	html = ""
	if strings.HasPrefix(watchPath, "m3u8/menu") {
		menupath := pathsplit(watchPath, 2)
		menu := global.menu.get(menupath, nil)
		log.Printf("pages: %s", menupath)
		if menu != nil {
			html = fmt.Sprintf(`<a target=_blank href="/menu/%s">%s</a>`, menu.path, menu.Desc)
		}
	}
	if html == "" {
		html = fmt.Sprintf(`<a href="%s">%s</a>`, watch, watch)
	}
	return
}

func (u user) getPageS() (pu usersS) {
	pu = usersS{
		Name: u.name,
		NameHref: "/user/"+u.name,
		Watch: u.watch,
		WatchHtml: getWatchHtml(u.watch, u.watchPath),
	}
	return
}

func usersPage(w io.Writer, path string) {
	m := global.user.shotall()
	users := []usersS{}
	for _, u := range m.m {
		users = append(users, u.getPageS())
	}
	renderIndex(w, "user", userlistPage(users))
}

func (m usermap) listPlayers(path string) (users []usersS) {
	for _, u := range m.m {
		if u.watchPath == path {
			users = append(users, u.getPageS())
		}
	}
	return
}

func (m usermap) countPlayers(path string) (n int) {
	log.Printf("count %s", path)
	for _, u := range m.m {
		if u.watchPath == path {
			n++
		}
	}
	return
}


