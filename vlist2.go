
package main

import (
	"sync"
	"fmt"
	"log"
	"errors"
	"sort"
)

type vlistV2Entry struct {
	Name string
	Pos int
}

type vlistV2EntriesArr []*vlistV2Entry

type vlistV2Node struct {
	Map map[string]*vlistV2Entry
	Arr vlistV2EntriesArr
	Desc string
	name string
}

type vlistV2 struct {
	m map[string]*vlistV2Node
	l *sync.Mutex
}

func loadVlistV2() (m *vlistV2) {
	m = &vlistV2{}
	m.l = &sync.Mutex{}
	m.m = map[string]*vlistV2Node{}
	return
}

func (m *vlistV2Node) log(format string, v ...interface{}) {
	str := fmt.Sprintf(format, v...)
	log.Printf("vlist %s: %s", m.name, str)
}

func (m *vlistV2) log(format string, v ...interface{}) {
	str := fmt.Sprintf(format, v...)
	log.Printf("vlist: %s", str)
}

func (m vlistV2EntriesArr) Len() int {
	return len(m)
}

func (m vlistV2EntriesArr) Swap(i,j int) {
	m[i],m[j] = m[j],m[i]
}

func (m vlistV2EntriesArr) Less(i,j int) bool {
	return m[i].Pos < m[j].Pos
}

func (m *vlistV2Node) sort() {
	var ret vlistV2EntriesArr
	for _, node := range m.Map {
		ret = append(ret, node)
	}
	sort.Sort(ret)
	for i, _ := range ret {
		ret[i].Pos = i
	}
	m.Arr = ret
}

func (m *vlistV2Node) del(names []string) (ret []string) {
	for _, name := range names {
		if _, ok := m.Map[name]; ok {
			ret = append(ret, name)
		}
	}
	for _, name := range ret {
		delete(m.Map, name)
	}
	m.sort()
	return
}

func (m *vlistV2Node) add(names []string) (ret []string) {
	for _, name := range names {
		if _, ok := m.Map[name]; !ok {
			e := &vlistV2Entry{Pos: len(m.Arr), Name: name}
			m.Map[name] = e
			m.Arr = append(m.Arr, e)
			ret = append(ret, name)
		}
	}
	return
}

func (m *vlistV2Node) updown(name string, args form) (err error, to int) {
	node, ok := m.Map[name]
	if !ok {
		err = errors.New(fmt.Sprintf("node %s not found", name))
		return
	}
	switch args.str("to") {
	case "up":
		to = node.Pos - 1
	case "down":
		to = node.Pos + 1
	default:
		err = errors.New(fmt.Sprintf("move to where?")) 
	}
	if to < 0 || to > len(m.Arr) {
		err = errors.New(fmt.Sprintf("node %s pos %d out of range", name, node.Pos)) 
		return
	}
	m.Arr[node.Pos],m.Arr[to] = m.Arr[to],m.Arr[node.Pos]
	node.Pos = to
	return
}

func (m *vlistV2) add(args form) (name string) {
	m.l.Lock()
	defer m.l.Unlock()

	for {
		name = randsha1()
		if _, ok := m.m[name]; ok {
			continue
		}
		break
	}
	node := &vlistV2Node{}
	node.Map = map[string]*vlistV2Entry{}
	node.Arr = vlistV2EntriesArr{}
	node.Desc = name
	node.name = name
	m._modify(node, args)
	m.m[name] = node

	return
}

func (m *vlistV2) _modify(node *vlistV2Node, args form) (err error, ret interface{}) {
	names := args.strs("names")
	names = append(names, args.strs("name")...)

	if _, ok := args.str2("do2"); ok {
		switch args.str("do2") {
		case "del":
			ret = node.del(names)
		case "add":
			ret = node.add(names)
		case "updown":
			err, ret = node.updown(args.str("name"), args)
		default:
			err = errors.New("unknown do2 op")
		}
	}

	if str, ok := args.str2("desc"); ok {
		if str != "" {
			node.Desc = str	
		} else {
			err = errors.New(fmt.Sprintf("desc is empty"))
		}
	}

	ret = hash{"ret":"ok"}
	return
}

func (m *vlistV2) modify(name string, args form) (err error, ret interface{}) {
	m.l.Lock()
	defer m.l.Unlock()

	node, ok := m.m[name]
	if !ok {
		err = errors.New(fmt.Sprintf("node '%s' not found", name))
		return
	}
	
	err, ret = m._modify(node, args)
	return
}

func (m *vlistV2) del(names []string) (ret []string) {
	m.l.Lock()
	defer m.l.Unlock()
	for _, name := range names {
		_, ok := m.m[name]
		if ok {
			ret = append(ret, name)
		}
	}
	for _, name := range ret {
		delete(m.m, name)
	}
	return
}

func (m *vlistV2) post(args form) (err error, ret interface{}) {
	var names []string
	switch args.str("do") {
	case "add":
		name := m.add(args)
		ret = hash{"ret":"ok", "name":name}
	case "del":
		names = args.strs("names")
		names = append(names, args.strs("name")...)
		del := m.del(names)
		ret = hash{"ret":"ok", "del":del, "count":len(del)}
	case "modify":
		err, ret = m.modify(args.str("name"), args)
	case "viewall":
		err, ret = m.viewall(args)
	case "viewone":
		err, ret = m.viewone(args)
	default:
		err = errors.New(fmt.Sprintf("unknown operation"))
	}
	return
}

type vlistV2ViewNode struct {
	Name,Desc string
	Count int
	Empty bool
	Nodes vlistV2EntriesArr
}

type vlistV2View struct {
	Count int
	Empty bool
	Nodes []vlistV2ViewNode
}

func (m *vlistV2) viewone(args form) (err error, ret vlistV2ViewNode) {
	m.l.Lock()
	defer m.l.Unlock()
	
	var node *vlistV2Node
	var ok bool

	name := args.str("name")
	if node, ok = m.m[name]; !ok {
		err = errors.New(fmt.Sprintf("node '%s' not found", name))
		return
	}
	
	ret.Name = name
	ret.Desc = node.Desc
	ret.Nodes = node.Arr
	ret.Count = len(node.Arr)
	ret.Empty = ret.Count == 0

	return
}

func (m *vlistV2) viewall(args form) (err error, ret vlistV2View) {
	m.l.Lock()
	defer m.l.Unlock()

	for _, node := range m.m {
		ret.Nodes = append(ret.Nodes, vlistV2ViewNode{
			Name: node.name,
			Desc: node.Desc,
			Count: len(node.Arr),
		})
	}
	ret.Count = len(ret.Nodes)
	ret.Empty = ret.Count == 0

	return
}
