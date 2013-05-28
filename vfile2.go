
package main

import (
	"sync"
	"path/filepath"
	"io"
	"os"
	"errors"
	"log"
	"strings"
	"fmt"
)

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

func loadVfileFromCsv(path string) (m *vfileV2) {
	m = &vfileV2{}
	m.m = map[string]*vfileV2Node{}
	m.l = &sync.Mutex{}

	i := 0
	readLines(path, func (line string) error {
		arr := strings.Split(line, ",")
		if len(arr) < 2 {
			return nil
		}
		desc := arr[0]
		url := arr[1]
		if strings.Contains(url, "m3u8") {
			name := fmt.Sprintf("file%d", i)
			vpath := filepath.Join("vfiles", name)
			v := vfileNewVProxy(url, vpath)
			v.name = name
			v.Desc = desc
			m.m[name] = v
		}
		i++
		return nil
	})
	return
}

func loadVfileV2(prefix string) (m *vfileV2) {
	m = &vfileV2{}
	m.m = map[string]*vfileV2Node{}
	m.l = &sync.Mutex{}
	file, err := os.Open("vfiles")
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
		v := vfileFromPath(filepath.Join("vfiles", name))
		if v != nil {
			m.m[name] = v
		}
	}
	return
}

func (m *vfileV2) lockDo(name string, cb func (*vfileV2Node) error) (err error) {
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

type lockAddCb func(string) (error, *vfileV2Node)

func (m *vfileV2) lockAdd(cb lockAddCb) (err error, name string) {
	m.l.Lock()
	defer m.l.Unlock()
	
	for {
		name = randsha1()
		if _, ok := m.m[name]; ok {
			continue
		}
		break
	}

	var node *vfileV2Node
	err, node = cb(name)
	if err != nil {
		return
	}

	node.name = name
	m.m[name] = node
	return
}

func (m *vfileV2) del(names []string) (ret []string) {
	m.l.Lock()
	defer m.l.Unlock()
	for _, name := range names {
		node, ok := m.m[name]
		if ok {
			node.stopAndRemove()
			ret = append(ret, name)
		}
	}
	for _, name := range ret {
		delete(m.m, name)	
	}
	return
}

func (m *vfileV2) modify(name string, args form) (err error) {
	err = m.lockDo(name, func (node *vfileV2Node) (err error) {
		err = node.modify(args)
		return
	})
	return
}

func (m *vfileV2) newDownload(url string) (err error, name string) {
	err, name = m.lockAdd(func (name string) (err error, node *vfileV2Node) {
		node, err = vfileNewDownload(url, filepath.Join("vfiles", name))
		return
	})
	return
}

func (m *vfileV2) newVProxy(url string) (err error, name string) {
	err, name = m.lockAdd(func (name string) (err error, node *vfileV2Node) {
		node = vfileNewVProxy(url, filepath.Join("vfiles", name))
		return
	})
	return
}

func (m *vfileV2) newUpload(filename string, r io.Reader, length int64) (err error, name string) {
	err, name = m.lockAdd(func (name string) (err error, node *vfileV2Node) {
		node = vfileNewUpload(filename, r, length, filepath.Join("vfiles", name))
		return
	})
	return
}

func (m *vfileV2) downloadMany(urls []string) (added []hash) {
	for _, url := range urls {
		err, name := m.newDownload(url)
		if err != nil {
			added = append(added, hash{
				"IsErr": true,
				"Err": fmt.Sprintf("%s", err),
			})
		} else {
			added = append(added, hash{
				"Name": name,
			})
		}
	}
	return
}

func (m *vfileV2) post(args form) (err error, ret interface{}) {
	switch args.str("do") {
	case "add":
		urls := args.strs("url")
		urls = append(urls, args.strs("urls")...)
		added := m.downloadMany(urls)
		if len(added) == 0 {
			err = errors.New(fmt.Sprint("no url found"))
			return
		}
		ret = hash{"ret": "ok", "add": added, "count": len(added)}
	case "del":
		names := args.strs("name")
		names = append(names, args.strs("names")...)
		del := m.del(names)
		ret = hash{"ret": "ok", "del": del, "count": len(del)}
	case "viewone":
		err, ret = m.viewone(args.str("name"), args)
	case "viewall":
		err, ret = m.viewall(args)
	default:
		err = errors.New("unknown operation")
	}
	return
}

type vfileGet struct {
	Nodes []vfileV2NodeInfo
}

func (m *vfileV2) viewall(args form) (err error, g vfileGet) {
	m.l.Lock()
	defer m.l.Unlock()
	for _, v := range m.m {
		g.Nodes = append(g.Nodes, v.info())
	}
	return
}

func (m *vfileV2) viewone(name string, args form) (err error, info vfileV2NodeInfo) {
	err = m.lockDo(name, func (node *vfileV2Node) (err error) {
		info = node.info()
		return
	})
	return
}

func testvfile1(a []string) {
	loadVfileFromCsv("tvlistxml/sample2")
}
