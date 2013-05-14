
package main

import (
	"sync"
	"strings"
	"time"
	"io"
	"log"
	"net/http"
)

/*
vm vlist create
vm vlist info [name]
vm vlist del [name]
vm vlist set [name] [opts..]
*/

type vlistV2Node struct {
	Text string
	Pairs []textpair
	Type string
	Desc string
	Dur time.Duration
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

func (m *vlistV2) create() (name string) {
	m.l.Lock()
	defer m.l.Unlock()
	for {
		name = randsha1()
		if _, ok := m.m[name]; !ok {
			break
		}
	}
	node := &vlistV2Node{}
	m.m[name] = node
	return
}

type textpair struct {
	line, vfile string
}

func splitLines(text string) (lines []string) {
	for _, s := range strings.Split(text, "\n") {
		lines = append(lines, s)
	}
	return
}

func parseVlistFromText(text string) (pairs []textpair) {
	for _, line := range splitLines(text) {
		v, err := vm.vfile.info(line)
		if err == nil {
			pairs = append(pairs, textpair{line, v.name})
		}
	}
	return
}

func (m *vlistV2) set(name string, args form) {
	m.l.Lock()
	defer m.l.Unlock()
	v, ok := m.m[name]
	if !ok {
		return
	}
	text := args.str("text")
	if text != "" {
		v.Pairs = parseVlistFromText(text)
	}
}

func (m *vlistV2) rm(name string) {
	m.l.Lock()
	defer m.l.Unlock()
	delete(m.m, name)
}

type vlistRow1 struct {
	Name string
	Line string
	Desc string
	Geostr string
	Sizestr string
	Durstr string
	Statstr string
}

type vlistRow2 struct {
	Name string
	Desc string
	Count int
	Durstr string
}

type vlistView2 struct {
	Rows []vlistRow2
}

type vlistView1 struct {
	Rows []vlistRow1
	Src string
	TotDur string
	TotSize string
	IsLive bool
	IsNormal bool
	NotFound bool
	CanSort bool
	HideEdit bool
	ShowLine bool
	CheckDel bool
	ShowStat bool
	ShowSel bool
	ColSpan int
}

func (m *vlistV2) list2(args form) (view vlistView2) {
	m.l.Lock()
	defer m.l.Unlock()

	for name, v := range m.m {
		view.Rows = append(view.Rows, vlistRow2{
			Name: name,
			Desc: v.Desc,
			Count: len(v.Pairs),
			Durstr: tmdurstr(v.Dur),
		})
	}
	return
}

func (m *vlistV2) page1(name string, args form) (view vlistView1) {
	m.l.Lock()
	defer m.l.Unlock()

	view.Src = "/vlist/"+name

	if name == "test" {
		view.Rows = []vlistRow1 {
			{Line: "aaa", Desc: "aaa", Geostr: "11x33", Sizestr: "123M", Name: "xx"},
			{Line: "aaa", Desc: "aaa", Geostr: "11x33", Sizestr: "123M", Name: "yy"},
		}
	} else {
		v, ok := m.m[name]
		if !ok {
			view.NotFound = true
			return
		}

		view.ShowLine = true
		view.ShowSel = true
		view.ColSpan = 2
		for _, p := range v.Pairs {
			n, err := vm.vfile.info(p.vfile)
			if err != nil {
				continue
			}
			view.Rows = append(view.Rows, vlistRow1{
				Name: n.name,
				Desc: n.Desc,
				Sizestr: n.Sizestr(),
				Geostr: n.Geostr(),
				Durstr: tmdurstr(n.Dur),
				Line: p.line,
			})
		}
	}
	return
}

func (m *vlistV2) post(path string, r *http.Request, w io.Writer) {
	post := r.FormValue("post")
	log.Printf("vlist post: %s", post)
}

