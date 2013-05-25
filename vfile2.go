
package main

import (
	"sync"
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

func newVfileV2() (m *vfileV2) {
	m = &vfileV2{}
	m.m = map[string]*vfileV2Node{}
	m.l = &sync.Mutex{}
	return
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
		v := vfileFromPath(filepath.Join("vfile", name))
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

func (m *vfileV2) _newname() string {
	var name string
	for {
		name = randsha1()
		if _, ok := m.m[name]; !ok {
			break
		}
	}
	return name
}

func (m *vfileV2) info(name string) (info vfileV2NodeInfo, err error) {
	m.l.Lock()
	defer m.l.Unlock()
	for vname, v := range m.m {
		if vname == name {
			info = v.info()
			return
		}
	}
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

func (m *vfileV2) newDownload(url string) (name string, err error) {
	m.l.Lock()
	defer m.l.Unlock()
	name = m._newname()
	var v *vfileV2Node
	v, err = vfileNewDownload(url, filepath.Join("vfile", name))
	if err != nil {
		return
	}
	m.m[name] = v
	v.name = name
	return
}

func (m *vfileV2) newVProxy(url string) (name string) {
	m.l.Lock()
	defer m.l.Unlock()
	name = m._newname()
	var v *vfileV2Node
	v = vfileNewVProxy(url, filepath.Join("vfile", name))
	m.m[name] = v
	v.name = name
	return
}

func (m *vfileV2) newUpload(filename string, r io.Reader, length int64) (name string) {
	m.l.Lock()
	defer m.l.Unlock()
	name = m._newname()
	var v *vfileV2Node
	v = vfileNewUpload(filename, r, length, filepath.Join("vfile", name))
	m.m[name] = v
	v.name = name
	return
}

func (m *vfileV2) post(path string, args form, w io.Writer) {
	do := args.str("do")
	switch do {
	case "addone":
		name, err := m.newDownload(args.str("url"))
		if err != nil {
			jsonErr(w, err)
		} else {
			jsonWrite(w, hash{"ret":"ok", "nr":1, "names":[]string{name}})
		}
	case "addmany":
		urls := args.str("urls")
		added := []string{}
		for _, u := range splitLines(urls) {
			name,err := m.newDownload(u)
			if err == nil {
				added = append(added, name)
			}
		}
		jsonWrite(w, hash{"ret":"ok", "nr":len(added), "nodes":added})
	case "addtest":
		jsonWrite(w, hash{"nr":3, "nodes":[]string{"1", "2", "3"}})
	}
}


