
package main

import (
	"sync"
	"path/filepath"
	"log"
	"io"
	"errors"
	"fmt"
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
	name string
	Desc string
	Parent string
	Vlist string
	Type string
	Tags []string
}

type menuV2 struct {
	m map[string]*menuV2Node
	l *sync.Mutex
}

func loadMenuV2From() (m *menuV2) {
	return
}

func loadMenuV2(prefix string) (m *menuV2) {
	m = &menuV2{}
	m.m = map[string]*menuV2Node{}
	loadJson(filepath.Join(prefix, "menu2"), &m.m)
	for name, node := range m.m {
		node.name = name
	}
	m.l = &sync.Mutex{}
	if _, ok := m.m["root"]; !ok {
		m.m["root"] = &menuV2Node{}
	}
	m.m["root"].Desc = "主菜单"
	m.m["root"].Type = "dir"
	m.m["root"].name = "root"
	return
}

func (m *menuV2Node) log(format string, v ...interface{}) {
	str := fmt.Sprintf(format, v...)
	log.Printf("menu %s: %s", m.name, str)
}

func (m *menuV2) log(format string, v ...interface{}) {
	str := fmt.Sprintf(format, v...)
	log.Printf("menu: %s", str)
}

func (m menuV2Node) Typestr() string {
	switch m.Type {
	case "dir":
		return "子菜单"
	case "live":
		return "直播列表"
	case "normal":
		return "点播列表"
	}
	return "未知"
}

func (m *menuV2) save(prefix string) {
	m.l.Lock()
	defer m.l.Unlock()
	saveJson(filepath.Join(prefix, "menu2"), m.m)
}

func (m *menuV2) _rm(name string, del []string) ([]string) {
	del = append(del, name)
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
	_, ok := m.m[name]
	if !ok {
		return
	}
	del := m._rm(name, []string{})
	for _, n := range del {
		if n != "root" {
			delete(m.m, n)
		}
	}
	m.log("del %d entries", len(del))
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
	node.name = nname
	m.m[nname] = node
	node.Vlist = vm.vlist.create()
	node.log("create from %s", name)
	node.log("create vlist %s", node.Vlist)
	return nname
}

func (m *menuV2) info(name string) (node menuV2Node, err error) {
	m.l.Lock()
	defer m.l.Unlock()
	_node, ok := m.m[name]
	if !ok {
		err = errors.New("not found")
		return
	}
	node = *_node
	return
}

type menuPath1 struct {
	Name,Desc string
	End bool
	Typestr string
}

type menuView1 struct {
	Path []menuPath1
	Dir []menuPath1
	Parent string
	Name string
	Desc string
	NotFound bool
	CanUp bool
	CanEditTitle bool
	CanMove bool
	CanDel bool
	IsDir bool
	IsEmpty bool
}

func (m *menuV2) view1(name string, args form) (view menuView1) {
	if name == "test" {
		view.Name = "testname"
		view.Path = []menuPath1{
			{"xx", "根目录", false, ""},
			{"hah", "哈哈", false, ""},
			{"sas", "三级", true, ""},
		}
		view.Desc = "测试菜单"
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
			view.IsDir = true
		}
		return
	}

	node, err := m.info(name)
	parent := node
	if err != nil {
		view.NotFound = true
		return
	}

	view.Name = name;
	view.Parent = node.Parent
	view.CanUp = true
	view.CanMove = true
	view.CanDel = true
	if name == "root" {
		view.CanUp = false
		view.CanEditTitle = false
		view.CanMove = false
		view.CanDel = false
	}
	if node.Type == "dir" {
		view.IsDir = true
	}

	cur := name
	parr := []menuPath1{}
	for i := 0; i < 32; i++ {
		node, err = m.info(cur)
		parr = append(parr, menuPath1{
			Name:cur, Desc:node.Desc,
		})
		if err != nil || cur == "root" {
			break
		}
		cur = node.Parent
	}
	for i := len(parr)-1; i >= 0; i-- {
		view.Path = append(view.Path, parr[i])
	}
	if len(view.Path) > 0 {
		view.Path[len(view.Path)-1].End = true
	}

	arr := m.ls(name)
	for _, s := range arr {
		node, _ = m.info(s)
		view.Dir = append(view.Dir, menuPath1{
			Name:s, Desc:node.Desc, Typestr:node.Typestr()})
	}

	if len(view.Dir) == 0 {
		view.IsEmpty = true
	}
	parent.log("ls %d entries", len(view.Dir))

	return
}

func (m *menuV2) set(name string, args form) (err error) {
	m.l.Lock()
	defer m.l.Unlock()

	var node *menuV2Node
	var ok bool

	node, ok = m.m[name]
	if !ok {
		return
	}

	var str string

	str, ok = args.str2("desc")
	if ok {
		if str == "" {
			err = errors.New("标题不能为空")
			return
		}
		node.Desc = str
	}

	str, ok = args.str2("type")
	if ok {
		switch str {
		case "normal","live","dir":
			node.Type = str
		default:
			err = errors.New("类型错误")
			return
		}
	}

	return
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

func (m *menuV2) post(name string, args form, w io.Writer) {
	var err error
	var ok bool
	var str string
	var list []string

	switch args.str("do") {
	case "add":
		name2 := m.create(name)
		err = m.set(name2, args)
	case "modify":
		err = m.set(name, args)
	case "del":
		list, ok = args.strs2("list")
		if ok {
			for _, s := range list {
				m.rm(s)
			}
		}
		str, ok = args.str2("node")
		if ok {
			m.rm(str)
		}
	case "moveto":
	default:
	}
	if err != nil {
		jsonWrite(w, hash{"err":fmt.Sprintf("%v", err)})
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

