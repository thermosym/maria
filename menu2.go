
package main

import (
	"sync"
	"path/filepath"
	"log"
	"net/http"
	"io"
)

/*
vm menu ls [path]
vm menu rm [path]
vm menu create [path]
vm menu mv [oldpath] [newpath]
vm menu info [path]
vm menu set [path] opts..
*/

type menuV2Node struct {
	Desc string
	Parent string
	VList string
	Tags []string
}

type menuV2 struct {
	m map[string]*menuV2Node
	l *sync.Mutex
}

func loadMenuV2(prefix string) (m *menuV2) {
	m = &menuV2{}
	m.m = map[string]*menuV2Node{}
	loadJson(filepath.Join(prefix, "menu2"), &m.m)
	m.l = &sync.Mutex{}
	if _, ok := m.m["root"]; !ok {
		m.m["root"] = &menuV2Node{}
	}
	return
}

func (m *menuV2) save(prefix string) {
	m.l.Lock()
	defer m.l.Unlock()
	saveJson(filepath.Join(prefix, "menu2"), m.m)
}

func (m *menuV2) _rm(name string, del []string) ([]string) {
	if _, ok := m.m[name]; !ok {
		del = append(del, name)
	} else {
		return del
	}
	for nname, node := range m.m {
		if node.Parent == name {
			del = m._rm(nname, del)
		}
	}
	return del
}

func (m *menuV2) rm(name string) {
	m.l.Lock()
	defer m.l.Unlock()
	del := m._rm(name, []string{})
	for _, n := range del {
		delete(m.m, n)
	}
}

func (m *menuV2) ls(name string) (arr []string) {
	m.l.Lock()
	defer m.l.Unlock()
	if _, ok := m.m[name]; !ok {
		return
	}
	for nname, node := range m.m {
		if node.Parent == name {
			arr = append(arr, nname)
		}
	}
	return
}

func (m *menuV2) create(name string) string {
	m.l.Lock()
	defer m.l.Unlock()
	if _, ok := m.m[name]; !ok {
		return ""
	}
	var nname string
	for {
		nname = randsha1()
		if _, ok := m.m[nname]; ok {
			continue
		}
		break
	}
	node := &menuV2Node{}
	node.Desc = "new"
	node.Parent = name
	m.m[nname] = node
	return nname
}

func (m *menuV2) info(name string) menuV2Node {
	m.l.Lock()
	defer m.l.Unlock()
	node := *m.m[name]
	return node
}

type menuPath1 struct {
	Name,Desc string
	End bool
	Type string
}

type menuView1 struct {
	Path []menuPath1
	Name string
	Desc string
	NotFound bool
	IsEmpty bool
	IsDir bool
	Dir []menuPath1
	List vlistView1
}

func (m *menuV2) view1(name string, args form) (view menuView1) {
	m.l.Lock()
	defer m.l.Unlock()

	if name == "test" {
		view.Name = "testname"
		view.Path = []menuPath1{
			{"xx", "根目录", false, ""},
			{"hah", "哈哈", false, ""},
			{"sas", "三级", true, ""},
		}
		view.Desc = "测试菜单"
		view.List = vm.vlist.page1("test", args)
		switch args.str("m") {
		case "1":
			view.IsDir = false
		case "2":
			view.IsDir = true
			view.Dir = []menuPath1{
				{"xxa", "目录1", false, "直播"},
				{"xxb", "目录2", false, "点播"},
				{"xxc", "目录3", false, "目录"},
				{"xxd", "目录4", false, "目录"},
			}
		default:
			view.IsEmpty = true
			view.IsDir = true
		}
		return
	}

	node, ok := m.m[name]
	if !ok {
		view.NotFound = true
		return
	}
	view.List = vm.vlist.page1(node.VList, args)
	return
}

func (m *menuV2) set(name string, args form) {
}

func (m *menuV2) mv(src []string, dst string) {
	m.l.Lock()
	defer m.l.Unlock()
	if _, ok := m.m[dst]; !ok {
		return
	}
	for _, s := range src {
		if node, ok := m.m[s]; ok {
			node.Parent = dst
		}
	}
}

func (m *menuV2) post(path string, args *http.Request, w io.Writer) {
	name := pathsplit(path, 1)
	post := args.FormValue("post")
	log.Printf("menu %s: post", name)
	switch post {
	case "add":
		desc := args.FormValue("desc")
		log.Printf("menu %s add: desc => '%s'", name, desc)
		if desc == "" {
			jsonWrite(w, hash{"err":"标题不能为空", "id":"desc"})
			return
		}
	case "modify":
		desc := args.FormValue("desc")
		log.Printf("menu %s modify: desc => '%s'", name, desc)
		if desc == "" {
			jsonWrite(w, hash{"err":"标题不能为空", "id":"desc"})
			return
		}
	case "del":
		node := args.FormValue("node")
		list, ok := args.Form["list"]
		log.Printf("menu %s del: node => '%s'", name, node)
		if ok {
			log.Printf("menu %s del: list => '%s'", name, list)
		}
	case "moveto":
	default:
	}
}

func testMenuV2() {
	m := loadMenuV2("/tmp")
	log.Printf("load %v", m.m)
	m.save("/tmp")
	s := m.create("root")
	log.Printf("%v", s)
	s = m.create("root")
	log.Printf("%v", s)
	s = m.create("root")
	log.Printf("%v", s)
	m.create(s)
	m.create(s)
	m.create(s)
	log.Printf("ls %s: %v", s, m.ls(s))
	m.save("/tmp")
}

