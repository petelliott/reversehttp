package reversehttp

import (
	"net/http"
	"errors"
	"io"
	"bufio"
	"fmt"
	"bytes"
	"io/ioutil"
)

func NewRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return req, err
	}
	req.Header.Add("Upgrade", "PTTH/1.0")
	req.Header.Add("Connection", "Upgrade")
	return req, nil
}

func IsReverseHTTPResponse(resp *http.Response) bool {
	if resp == nil {
		return false
	}

	return resp.Header.Get("Upgrade") == "PTTH/1.0" &&
		resp.Header.Get("Connection") == "Upgrade" &&
		resp.StatusCode == http.StatusSwitchingProtocols
}

type response struct {
	writer io.Writer
	bodybuf *bytes.Buffer
	req *http.Request
	status int
	header http.Header
	headwritten bool
}

func newResponse(req *http.Request, writer io.Writer) *response {
	r := response{writer, new(bytes.Buffer), req, 0, http.Header{}, false}
	return &r
}

func (r *response) Header() http.Header {
	return r.header
}

func (r *response) Write(b []byte) (int, error) {
	if !r.headwritten {
		r.WriteHeader(http.StatusOK)
	}

	return r.bodybuf.Write(b)
}

func (r *response) WriteHeader(statusCode int) {
	if r.headwritten {
		return
	}

	r.status = statusCode

	r.headwritten = true
}

func (r *response) Flush() {
	resp := http.Response{
		StatusCode: r.status,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Request: r.req,
		Header: r.header,
		ContentLength: int64(r.bodybuf.Len()),
		Body: ioutil.NopCloser(r.bodybuf),
	}

	resp.Write(r.writer)
}

func ReverseResponse(resp *http.Response, handler http.Handler) error {
	if !IsReverseHTTPResponse(resp) {
		return errors.New(
			"response is not a valid reverse http upgrade response")
	}

	breader := resp.Body
	bwriter := resp.Body.(io.Writer)

	// TODO: http persistent connections

	req, err := http.ReadRequest(bufio.NewReader(breader))
	if err != nil {
		return fmt.Errorf("error reading request: %v", err)
	}
	w := newResponse(req, bwriter)
	handler.ServeHTTP(w, req)
	w.Flush()
	resp.Body.Close()
	return nil
}

func Reverse(url string, handler http.Handler) error {
	req, err := NewRequest(url)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return ReverseResponse(resp, handler)
}

func ReverseFunc(url string, fun func(w http.ResponseWriter, r *http.Request)) error {
	return Reverse(url, http.HandlerFunc(fun))
}
