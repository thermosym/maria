
package main

import (
	"os"
	"log"
	"errors"
	"fmt"
	"net"
	"net/http"
	"io"
	"io/ioutil"
	"time"
)

func curl(url string) (body string, err error){
	var resp *http.Response
	resp, err = http.Get(url)
	if err != nil {
		return
	}
	var bodydata []byte
	bodydata, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	body = string(bodydata)
	return
}

func curldata(url string) (body []byte, err error) {
	var resp *http.Response
	resp, err = http.Get(url)
	if err != nil {
		return
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	return
}

func curl2(url string) (err error, r io.Reader, length int64) {
	var req *http.Request
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header = http.Header {
		"Accept" : {"*/*"},
		"User-Agent" : {"curl/7.29.0"},
	}

	var resp *http.Response
	tr := &http.Transport {
		DisableCompression: true,
		Dial: func (netw, addr string) (net.Conn, error) {
			return net.DialTimeout(netw, addr, time.Second*40)
		},
	}
	client := &http.Client{
		Transport: tr,
	}
	resp, err = client.Do(req)
	if err != nil {
		return
	}

	r = resp.Body
	length = resp.ContentLength
	return
}

func curl3(url, path string, cb ioCopyCb) (err error, speed,size int64) {
	var w *os.File
	w, err = os.Create(path)
	if err != nil {
		err = errors.New(fmt.Sprintf("create %s failed", path))
		return
	}
	defer w.Close()

	var r io.Reader
	var length int64
	err, r, length = curl2(url)
	if err != nil {
		err = errors.New(fmt.Sprintf("curl2 %s failed", url))
		return
	}

	err, speed, size = ioCopy(r, length, w, cb)
	return
}

func speedstr(s int64) string {
	return fmt.Sprintf("%s/s", sizestr(s))
}

type iocopyStat struct {
	per float64
	dur time.Duration
	size, speed int64
}

type ioCopyCb func (st iocopyStat) error

func ioCopy(r io.Reader, length int64, w io.Writer, cb ioCopyCb) (err error, avgspeed,size int64) {
	var n, speed int64
	begin := time.Now()
	start := time.Now()

	for {
		n, err = io.CopyN(w, r, 64*1024)
		size += n
		speed += n
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}

		since := time.Since(start)
		if since > time.Second {
			var per float64
			if length > 0 {
				per = float64(size)/float64(length)
			}
			err = cb(iocopyStat{per, since, size, speed})
			if err != nil {
				return
			}
			start = time.Now()
			speed = 0
		}
	}

	dur := time.Since(begin)
	dur2 := int64(dur)/int64(time.Millisecond)
	if dur2 > 0 {
		avgspeed = size*1000/dur2
	}

	return
}

func testCurl3(_a []string) {
	curl3(
		"http://dldir1.qq.com/qqfile/qq/QQ2013/2013Beta3/6565/QQ2013Beta3.exe",
		"/tmp/qq.exe",
		func (st iocopyStat) error {
			log.Printf("%v %v %v", st.per, sizestr(st.size), speedstr(st.speed))
			return nil
		})
}

