
package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"log"
	"strings"
)

type xmlS struct {
	Tv []tv `xml:"tvlist>tv"`
}

type tv struct {
	Name string `xml:"name"`
	Url []string `xml:"url"`
}

func main() {

	fout, _ := os.Create("list")

	log.SetOutput(os.Stdout)
	var nodes xmlS
	var out xmlS
	f, _ := os.Open("tvlist.xml")
	dec := xml.NewDecoder(f)
	dec.Decode(&nodes)
	for _, t := range nodes.Tv {
		resurl := ""
		for _, u := range t.Url {
			//log.Printf("  %s", u)
			if strings.Contains(u, "m3u8") {
				resurl = u
				break
			}
		}
		if resurl != "" {
			log.Printf("%s %s", t.Name, resurl)
			fmt.Fprintf(fout, "%s,%s\n", t.Name, resurl)
			out.Tv = append(out.Tv, tv{Name:t.Name, Url:[]string{resurl}})
		}
	}

	fout.Close()
}

