
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
)

type user struct {
	name string
	cpuinfo,sysinfo,meminfo string
	app,appver string
	watch,watchPath string
}

type usermap struct {
	m map[string]*user
}

func loadUsermap() (m usermap) {
	m.m = map[string]*user{}
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
	if r.FormValue("watch") != "" {
		u.watch = r.FormValue("watch")
		u.watchPath = getWatchPath(u.watch)
		log.Printf("user %s: watch %s", name, u.watchPath)
	}
	if r.FormValue("app") != "" {
		u.app = r.FormValue("app")
	}
	if r.FormValue("appver") != "" {
		u.appver = r.FormValue("appver")
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

func (u user) getPageS() (pu usersS) {
	html := ""
	if strings.HasPrefix(u.watchPath, "m3u8/menu") {
		menupath := pathsplit(u.watchPath, 2)
		menu := global.menu.get(menupath, nil)
		log.Printf("pages: %s", menupath)
		if menu != nil {
			html = fmt.Sprintf(`<a target=_blank href="/menu/%s">%s</a>`, menu.path, menu.Desc)
		}
	}
	if html == "" {
		html = fmt.Sprintf(`<a href="%s">%s</a>`, u.watch, u.watch)
	}
	pu = usersS{
		Name: u.name,
		NameHref: "/user/"+u.name,
		Watch: u.watch,
		WatchHtml: html,
		Time: "N/a",
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

