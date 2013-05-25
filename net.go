
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

func curl(url string, opts... interface{}) (body string, err error){
	var _body []byte
	_body, err = curldata(url, opts...)
	body = string(_body)
	return
}

func curldata(url string, opts... interface{}) (body []byte, err error) {
	var r io.Reader
	err, r, _, _ = curl2(url, opts...)
	if err != nil {
		return
	}
	body, err = ioutil.ReadAll(r)
	if err != nil {
		return
	}
	return
}

func curl2(url string, opts... interface{}) (err error, r io.Reader, length int64, conn net.Conn) {
	var req *http.Request
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header = http.Header {
		"Accept" : {"*/*"},
		"User-Agent" : {"curl/7.29.0"},
	}

	connTimeout := -1
	for _, opt := range opts {
		switch opt.(type) {
		case string:
			s := opt.(string)
			fmt.Sscanf(s, "connTimeout=%d", &connTimeout)
			fmt.Sscanf(s, "timeout=%d", &connTimeout)
		}
	}

	//log.Printf("curl %s timeout %d", url, timeout)

	var resp *http.Response

	tr := &http.Transport {
		DisableCompression: true,
		Dial: func (netw, addr string) (net.Conn, error) {
			if connTimeout != -1 {
				dur := time.Duration(connTimeout)*time.Second
				var e error
				conn, e = net.DialTimeout(netw, addr, dur)
				return conn, e
			}
			return net.Dial(netw, addr)
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

func curl3(url, path string, cb ioCopyCb, opts... interface{}) (err error, speed,size int64) {
	var w *os.File
	w, err = os.Create(path)
	if err != nil {
		err = errors.New(fmt.Sprintf("create %s failed", path))
		return
	}
	defer w.Close()

	var r io.Reader
	var length int64
	var conn net.Conn

	err, r, length, conn = curl2(url, opts...)
	if err != nil {
		err = errors.New(fmt.Sprintf("curl2 %s failed", url))
		return
	}

	err, speed, size = ioCopy(r, length, w, cb, conn, opts...)
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

func ioCopy(
	r io.Reader, length int64, w interface{},
	cb ioCopyCb, conn net.Conn,
	opts... interface{},
) (err error, avgspeed,size int64) {

	readTimeout := -1
	for _, opt := range opts {
		switch opt.(type) {
		case string:
			s := opt.(string)
			fmt.Sscanf(s, "readTimeout=%d", &readTimeout)
			fmt.Sscanf(s, "timeout=%d", &readTimeout)
		}
	}

	var n, speed int64
	begin := time.Now()
	start := time.Now()

	for {
		if readTimeout != -1 && conn != nil {
			conn.SetReadDeadline(
				time.Now().Add(time.Duration(readTimeout)*time.Second),
			)
		}

		n, err = io.CopyN(w.(io.Writer), r, 64*1024)
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
	cb(iocopyStat{1, dur, size, avgspeed})

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

func testHttpTimeout(a []string) {
	url := "http://www.kernel.org/pub/linux/kernel/v3.x/linux-3.9.3.tar.xz"
	var req *http.Request
	var err error
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("%v", err)
		return
	}
	req.Header = http.Header {
		"Accept" : {"*/*"},
		"User-Agent" : {"curl/7.29.0"},
	}

	to := time.Second*10

	var resp *http.Response
	tr := &http.Transport {
		DisableCompression: true,
		Dial: func (netw, addr string) (net.Conn, error) {
			c, err := net.DialTimeout(netw, addr, to)
			c.SetDeadline(time.Now().Add(to))
			return c, err
		},
	}
	client := &http.Client{
		Transport: tr,
	}
	resp, err = client.Do(req)
	if err != nil {
		log.Printf("%v", err)
		return
	}

	var r io.Reader
	var length int64
	r = resp.Body
	length = resp.ContentLength
	log.Printf("got length %d", length)

	_, err = ioutil.ReadAll(r)
	log.Printf("read done %v", err)

	return
}

