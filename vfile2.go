
package main

import (
	"sync"
	"strings"
	"path/filepath"
	"io"
	"os"
	"errors"
	"log"
)

/*
vm vfile ls
vm vfile info [path]
vm vfile create [desc]
vm vfile download [path] [url]
*/

type vfileV2 struct {
	m map[string]*vfileV2Node
	l *sync.Mutex
}

func loadVfileV2(prefix string) (m *vfileV2) {
	m = &vfileV2{}
	m.m = map[string]*vfileV2Node{}
	m.l = &sync.Mutex{}
	file, err := os.Open("vfile")
	if err != nil {
		return
	}
	dirs, err := file.Readdir(0)
	if err != nil {
		return
	}
	for _, fi := range dirs {
		if !fi.IsDir() {
			continue
		}
		name := fi.Name()
		log.Printf("vlist: load %v", name)
		v := loadVfileV2Node(filepath.Join("vfile", name), name)
		if v != nil {
			m.m[name] = v
		}
	}
	return
}

func (m *vfileV2) ls() (arr []string) {
	m.l.Lock()
	defer m.l.Unlock()
	for name, _ := range m.m {
		arr = append(arr, name)
	}
	return
}

func (m *vfileV2) rm(name string) {
	m.l.Lock()
	defer m.l.Unlock()
	v, ok := m.m[name]
	if !ok {
		return
	}
	delete(m.m, name)
	v.rm()
}

func (m *vfileV2) create() string {
	m.l.Lock()
	defer m.l.Unlock()
	var name string
	for {
		name = randsha1()
		if _, ok := m.m[name]; !ok {
			break
		}
	}
	node := &vfileV2Node{}
	node.l = &sync.Mutex{}
	node.Stat = "init"
	node.path = filepath.Join("vfile", name)
	node.name = name
	m.m[name] = node
	return name
}

func (m *vfileV2) info(name string) (r vfileV2Node, err error) {
	m.l.Lock()
	defer m.l.Unlock()
	for vname, v := range m.m {
		if vname == name {
			r = *v
			return
		}
		if v.Src == name {
			r = *v
			return
		}
	}
	r.Name = name
	err = errors.New("not found")
	return
}

func (m *vfileV2) set(name string, args form) {
	m.l.Lock()
	defer m.l.Unlock()
	v, ok := m.m[name]
	if !ok {
		return
	}
	v.set(args)
}

func (m *vfileV2) download(name,url string) (err error) {
	m.l.Lock()
	defer m.l.Unlock()
	v, ok := m.m[name]
	if !ok {
		return
	}
	err = v.downloadCheck(url)
	if err != nil {
		return
	}
	go v.download(url)
	return
}

func (m *vfileV2) upload(name string, filename string, r io.Reader, length int64) {
	m.l.Lock()
	defer m.l.Unlock()
	v, ok := m.m[name]
	if !ok {
		return
	}
	go v.upload(filename, r, length)
}

func (m *vfileV2) post(path string, args form, w io.Writer) {
	post := args.str("post")
	switch post {
	case "download":
		name := m.create()
		err := m.download(name, args.str("url"))
		if err != nil {
			jsonErr(w, err)
		} else {
			jsonWrite(w, hash{"list":name})
			log.Printf("ret")
		}
	}
}

type vfileWatchRow1 struct {
	Statstr,Src,Name,Type string
}

type vfileWatch1 struct {
	List []vfileWatchRow1
}

func (m *vfileV2) watch1(args form) (view vfileWatch1) {
	list := args.str("list")
	if list == "" {
		return
	}
	for _, name := range strings.Split(list, ",") {
		v, _ := m.info(name)
		row := vfileWatchRow1{
			Statstr: v.Statstr(),
			Src: v.Src,
			Name: name,
			Type: v.Typestr(),
		}
		view.List = append(view.List, row)
	}
	return
}

type vfileOne1 struct {
	vfileV2Node
}

func (m *vfileV2) one1(name string, args form) (view vfileOne1){
	v, _ := m.info(name)
	view = vfileOne1{v}
	return
}

func (m *vfileV2) page1(args form) (view vlistView1) {
	m.l.Lock()
	defer m.l.Unlock()
	view.CanSort = true
	view.CheckDel = true
	view.ShowStat = true
	view.ShowSel = false
	for name, v := range m.m {
		view.Rows = append(view.Rows, vlistRow1 {
			Statstr: v.Statstr(),
			Desc: v.Desc,
			Geostr: v.Geostr(),
			Sizestr: v.Sizestr(),
			Name: name,
		})
	}
	view.RowsEmpty = len(view.Rows) == 0
	return
}

