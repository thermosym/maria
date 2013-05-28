
package main

import (
	"sync"
	"path/filepath"
	"log"
	"errors"
	"fmt"
)

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
	m.m["root"].Desc = "root"
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

func (m *menuV2) save(prefix string) {
	m.l.Lock()
	defer m.l.Unlock()
	saveJson(filepath.Join(prefix, "menu2"), m.m)
}

func (m *menuV2) lockDo(name string, cb func (*menuV2Node) error) (err error) {
	m.l.Lock()
	defer m.l.Unlock()
	node, ok := m.m[name]
	if !ok {
		err = errors.New(fmt.Sprintf("node %s not found", name))
		return
	}
	err = cb(node)
	return
}

func (m *menuV2) _del(parent string, del []string) ([]string) {
	if _, ok := m.m[parent]; !ok {
		return del
	}
	del = append(del, parent)
	for name, node := range m.m {
		if node.Parent == parent {
			del = m._del(name, del)
		}
	}
	return del
}

func (m *menuV2) del(names []string) (del []string) {
	m.l.Lock()
	defer m.l.Unlock()
	for _, name := range names {
		del = m._del(name, del)
	}
	for _, name := range del {
		if name != "root" {
			delete(m.m, name)
		}
	}
	return
}

func (m *menuV2) add(args form) (err error, name string) {
	parent := args.str("parent")
	err = m.lockDo(parent, func (_ *menuV2Node) (err error) {
		node := &menuV2Node{}
		node.Parent = parent
		err = m._modify(node, args)
		if err != nil {
			return
		}
		if node.Type == "" {
			err = errors.New(fmt.Sprintf("type is empty"))
			return
		}
		if node.Desc == "" {
			err = errors.New(fmt.Sprintf("desc is empty"))
			return
		}

		for {
			name = randsha1()
			if _, ok := m.m[name]; ok {
				continue
			}
			break
		}
		node.name = name
		m.m[name] = node
		return
	})
	return
}

func (m *menuV2) _modify(node *menuV2Node, args form) (err error) {
	var str string
	var ok bool

	str, ok = args.str2("desc")
	if ok {
		if str == "" {
			err = errors.New("title cannot be empty")
			return
		}
		node.Desc = str
	}

	str, ok = args.str2("type")
	if ok {
		switch str {
		case "normal","live","dir","vlink","mlink":
			node.Type = str
		default:
			err = errors.New(fmt.Sprintf("type '%s' is invalid", str))
			return
		}
	}

	return
}

func (m *menuV2) modify(name string, args form) (err error) {
	err = m.lockDo(name, func (node *menuV2Node) (err error) {
		err = m._modify(node, args)
		return
	})
	return
}

func (m *menuV2) moveto(src []string, dst string) (err error, move []string) {
	m.l.Lock()
	defer m.l.Unlock()
	if _, ok := m.m[dst]; !ok {
		err = errors.New(fmt.Sprintf("dst %s not found", dst))
		return
	}
	for _, s := range src {
		if node, ok := m.m[s]; ok {
			move = append(move, s)
			node.Parent = dst
		}
	}
	return
}

type menuPath1 struct {
	Name,Desc,Type string
	End bool
}

type menuView1 struct {
	Path []menuPath1
	Dir []menuPath1
	DirEmpty bool
	Parent string
	Name string
	Desc string
	Type string
	CanUp bool
	CanEditTitle bool
	CanMove bool
	CanDel bool
	IsDir bool
}

func (m *menuV2) get(args form) (err error, view menuView1) {
	m.l.Lock()
	defer m.l.Unlock()

	var node,node2 *menuV2Node
	var name,name2 string
	var ok bool

	name, ok = args.str2("name")
	if !ok {
		err = errors.New(fmt.Sprintf("field 'name' not found"))
		return
	}
	node, ok = m.m[name]
	if !ok {
		err = errors.New(fmt.Sprintf("menu node '%s' not found", name))
		return
	}

	view.Name = name
	view.Desc = node.Desc
	view.Type = node.Type
	view.Parent = node.Parent

	if name != "root" {
		view.CanUp = true
		view.CanMove = true
		view.CanDel = true
	} else {
		view.CanEditTitle = true
	}
	if node.Type == "dir" {
		view.IsDir = true
	}

	name2 = name
	var arr []menuPath1
	for i := 0; i < 32; i++ {
		node, ok = m.m[name2]
		if !ok {
			break
		}
		arr = append(arr, menuPath1{
			Name:name2, Desc:node.Desc,
		})
		if name2 == "root" {
			break
		}
		name2 = node.Parent
	}
	for i := len(arr)-1; i >= 0; i-- {
		view.Path = append(view.Path, arr[i])
	}
	if len(view.Path) > 0 {
		view.Path[len(view.Path)-1].End = true
	}

	for _, node2 = range m.m {
		if node2.Parent == name {
			view.Dir = append(view.Dir, menuPath1{
				Name:node2.name, Desc:node2.Desc, Type:node2.Type})
		}
	}

	if len(view.Dir) == 0 {
		view.DirEmpty = true
	}

	return
}

func (m *menuV2) post(args form) (err error, ret interface{}) {
	var name string
	var names []string

	name = args.str("name")

	switch args.str("do") {
	case "add":
		err, name = m.add(args)
		if err != nil {
				return
		}
		ret = hash{"ret":"ok", "name":name}
	case "modify":
		err = m.modify(name, args)
		if err != nil {
			return
		}
		ret = hash{"ret":"ok"}
	case "del":
		names = args.strs("names")
		names = append(names, args.strs("name")...)
		del := m.del(names)
		ret = hash{"ret":"ok", "del":del, "count":len(del)}
	case "moveto":
		err, names = m.moveto(names, args.str("dst"))
		ret = hash{"ret":"ok", "move":names, "count":len(names)}
	case "view":
		err, ret = m.get(args)
	default:
		err = errors.New(fmt.Sprintf("unknown operation"))
	}
	return
}

func testMenuV2() {
}

