
package main

import (
	"sync"
	"strings"
	"fmt"
	"time"
	"log"
)

/*
vm vlist create
vm vlist info [name]
vm vlist del [name]
vm vlist set [name] [opts..]
*/

type vlistV2Node struct {
	Text string
	Names []string
	Type string
	Desc string
	Dur time.Duration
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


func (m *vlistV2) create() (name string) {
	m.l.Lock()
	defer m.l.Unlock()
	for {
		name = randsha1()
		if _, ok := m.m[name]; !ok {
			break
		}
	}
	node := &vlistV2Node{name:name}
	m.m[name] = node
	return
}

func splitLines(text string) (lines []string) {
	for _, s := range strings.Split(text, "\n") {
		lines = append(lines, s)
	}
	return
}

func (m *vlistV2) set(name string, args form) {
	m.l.Lock()
	defer m.l.Unlock()
	_, ok := m.m[name]
	if !ok {
		return
	}
}

func (m *vlistV2) rm(name string) {
	m.l.Lock()
	defer m.l.Unlock()
	delete(m.m, name)
}

