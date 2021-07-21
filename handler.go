package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
)

var _f = fmt.Sprintf

type _o = map[string]interface{}

var _these = make(map[*http.Request]*This)

type This struct {
	context.Context
	Event _o
}

func GetThis(r *http.Request) *This {
	return _these[r]
}

func _headers(_headers _o) map[string]string {
	headers := make(map[string]string)
	for k, v := range _headers {
		headers[k] = v.(string)
	}
	return headers
}

func _headersMV(_headers _o) map[string]string {
	headers := make(map[string]string)
	for k, v := range _headers {
		headers[k] = strings.Join(v.([]string), ",")
	}
	return headers
}

func _qs(_qs _o) string {
	if len(_qs) == 0 {
		return ""
	}
	var qs []string
	for k, v := range _qs {
		qs = append(qs, _f("%s=%s", k, v))
	}
	return _f("?%s", strings.Join(qs, "&"))
}

func _qsMV(_qs _o) string {
	if len(_qs) == 0 {
		return ""
	}
	var qs []string
	for k, vs := range _qs {
		for _, v := range vs.([]string) {
			qs = append(qs, _f("%s=%s", k, v))
		}
	}
	return _f("?%s", strings.Join(qs, "&"))
}

func _body(_body string, isB64Enc bool) []byte {
	body := []byte(_body)
	if isB64Enc {
		n, _ := base64.StdEncoding.Decode(body, body)
		body = body[:n]
	}
	return body
}

var _setCookie = strings.Split(""+
	"set-cookie Set-cookie sEt-cookie SEt-cookie seT-cookie SeT-cookie sET-cookie SET-cookie "+
	"set-Cookie Set-Cookie sEt-Cookie SEt-Cookie seT-Cookie SeT-Cookie sET-Cookie SET-Cookie "+
	"set-cOokie Set-cOokie sEt-cOokie SEt-cOokie seT-cOokie SeT-cOokie sET-cOokie SET-cOokie "+
	"set-COokie Set-COokie sEt-COokie SEt-COokie seT-COokie SeT-COokie sET-COokie SET-COokie "+
	"set-coOkie Set-coOkie sEt-coOkie SEt-coOkie seT-coOkie SeT-coOkie sET-coOkie SET-coOkie "+
	"set-CoOkie Set-CoOkie sEt-CoOkie SEt-CoOkie seT-CoOkie SeT-CoOkie sET-CoOkie SET-CoOkie "+
	"set-cOOkie Set-cOOkie sEt-cOOkie SEt-cOOkie seT-cOOkie SeT-cOOkie sET-cOOkie SET-cOOkie "+
	"set-COOkie Set-COOkie sEt-COOkie SEt-COOkie seT-COOkie SeT-COOkie sET-COOkie SET-COOkie", " ")

type meta struct {
	_mv                      bool
	method, path, script, qs string
	headers                  map[string]string
	body                     []byte
}

func (meta *meta) alb(ev _o) {
	path, _ := url.PathUnescape(ev["path"].(string))
	meta.method = ev["httpMethod"].(string)
	meta.path = path
	meta.script = os.Getenv("SCRIPT_NAME")
}

func _meta(ev _o) meta {
	meta := meta{body: _body(ev["body"].(string), ev["isBase64Encoded"].(bool))}
	if ev["version"] == "1.0" { // api v1
	} else if ev["version"] == "2.0" { // api v2
	} else if ev["headers"] != nil { // alb
		meta.alb(ev)
		meta._mv = false
		meta.headers = _headers(ev["headers"].(_o))
		meta.qs = _qs(ev["queryStringParameters"].(_o))
	} else { // alb mv
		meta.alb(ev)
		meta._mv = true
		meta.headers = _headersMV(ev["multiValueHeaders"].(_o))
		meta.qs = _qsMV(ev["multiValueQueryStringParameters"].(_o))
	}
	return meta
}

func _req(meta meta) *http.Request {
	var headers []string
	for k, v := range meta.headers {
		headers = append(headers, _f("%s: %s", k, v))
	}

	req, _ := http.ReadRequest(bufio.NewReader(bytes.NewReader(append(
		[]byte(_f(
			"%s %s%s HTTP/1.1\r\n%s\r\n\r\n",
			meta.method, meta.path, meta.qs,
			strings.Join(headers, "\r\n"),
		)),
		meta.body...,
	))))
	req.URL.Path = req.URL.Path[len(meta.script):]
	req.RemoteAddr = strings.TrimSpace(meta.headers["x-forwarded-for"][
		strings.LastIndex(meta.headers["x-forwarded-for"], ",")+1:])

	return req
}

func _rsp(meta meta, _rsp *http.Response) _o {
	body, _ := io.ReadAll(_rsp.Body)
	rsp := _o{
		"statusCode":        _rsp.StatusCode,
		"statusDescription": _rsp.Status,
		"body":              base64.StdEncoding.EncodeToString(body),
		"isBase64Encoded":   true,
	}

	if !meta._mv {
		headers := make(map[string]string)
		for k, vs := range _rsp.Header {
			if k == "Set-Cookie" {
				for i, v := range vs {
					headers[_setCookie[i]] = v
				}
				continue
			}
			headers[k] = strings.Join(vs, ",")
		}
		rsp["headers"] = headers
	} else {
		rsp["multiValueHeaders"] = _rsp.Header
	}

	return rsp
}

func Make(handler http.Handler) func(ctx context.Context, ev _o) (_o, error) {
	return func(ctx context.Context, ev _o) (_o, error) {
		log.Print("ev", ev)
		meta := _meta(ev)
		log.Print("meta", meta)
		w, req := httptest.NewRecorder(), _req(meta)
		log.Print("req", req)
		_these[req] = &This{ctx, ev}
		handler.ServeHTTP(w, req)
		delete(_these, req)
		rsp := _rsp(meta, w.Result())
		log.Print("rsp", rsp)
		return rsp, nil
	}
}
