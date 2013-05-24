
package main

import (
	"regexp"
	"errors"
	"fmt"
	"os"
)

func parseSohu(url string) (err error, m3u8url,body string, desc string) {

	var re *regexp.Regexp
	var ma []string

	body, err = curl(url)

	if err != nil {
		err = errors.New(fmt.Sprintf("curl index failed: %v", err))
		return
	}

	re, err = regexp.Compile(`<title>([^<]+)</title>`)
	ma = re.FindStringSubmatch(body)
	if len(ma) >= 2 {
		desc = gbk2utf(ma[1])
	}

	re, err = regexp.Compile(`var vid="([^"]+)"`)
	ma = re.FindStringSubmatch(body)

	if len(ma) != 2 {
		err = errors.New(fmt.Sprintf("sohu: cannot find vid: ma = %v", ma))
		return
	}

	vid := ma[1]

	m3u8url = "http://hot.vrs.sohu.com/ipad"+vid+".m3u8"
	body, err = curl(m3u8url)
	if err != nil {
		err = errors.New(fmt.Sprintf("fetch m3u8 failed: %v", err))
		return
	}

	if false {
		f, _ := os.Create("/tmp/m3u8")
		fmt.Fprintf(f, "%v", body)
		f.Close()
	}

	return
}


