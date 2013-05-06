
package main

import (
	"github.com/hoisie/mustache"

	"net/http"
	"log"
	"io"
	"fmt"
)

type user struct {
	name string
	cpuinfo,sysinfo,meminfo string
	app,appver string
	watch string
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

func (m usermap) interim(r *http.Request) {
	var name string
	name = r.FormValue("name")
	if name == "" {
		return
	}
	log.Printf("interim: %s", name)
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
	renderIndex(w,
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
	WatchHref, Watch string
	Time string
}

func userlistPage(users []usersS) string {
	return mustache.RenderFile("tpl/usersPage.html", map[string]interface{} {
		"livenr":len(users),
		"users":users,
	})
}

func usersPage(w io.Writer, path string) {
	m := global.user.shotall()
	users := []usersS{}
	for _, u := range m.m {
		users = append(users, usersS{
			Name: u.name,
			NameHref: "/user/"+u.name,
			Watch: u.watch,
			WatchHref: u.watch,
			Time: "N/a",
		})
	}
	renderIndex(w, userlistPage(users))
}

func (m usermap) listPlayers(_url string) (users []usersS) {
	for _, u := range m.m {
		if u.watch == _url {
			users = append(users, usersS{
				Name: u.name,
				NameHref: "/user/"+u.name,
				Watch: u.watch,
				WatchHref: u.watch,
				Time: "N/a",
			})
		}
	}
	return
}

func (m usermap) countPlayers(_url string) (n int) {
	for _, u := range m.m {
		if u.watch == _url {
			n++
		}
	}
	return
}

