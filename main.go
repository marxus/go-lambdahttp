package lambdaGoHTTP

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
)

var _f = fmt.Sprintf

type _o = map[string]interface{}

/* request parsing related */
func _body(_body string, decode bool) []byte {
	body := []byte(_body)
	if decode {
		n, _ := base64.StdEncoding.Decode(body, body)
		body = body[:n]
	}
	return body
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
	for k, vs := range _headers {
		for _, v := range vs.([]interface{}) {
			headers[k] += _f("%s,", v)
		}
		headers[k] = headers[k][:len(headers[k])-1]
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
	return strings.ReplaceAll(_f("?%s", strings.Join(qs, "&")), " ", "%20")
}

func _qsMV(_qs _o, escape bool) string {
	if len(_qs) == 0 {
		return ""
	}
	var qs []string
	for k, vs := range _qs {
		if escape {
			k = url.QueryEscape(k)
		}
		for _, v := range vs.([]interface{}) {
			if escape {
				v = url.QueryEscape(v.(string))
			}
			qs = append(qs, _f("%s=%s", k, v))
		}
	}
	return strings.ReplaceAll(_f("?%s", strings.Join(qs, "&")), " ", "%20")
}

/* response building related */
var _setCookie = strings.Split(""+
	"set-cookie Set-cookie sEt-cookie SEt-cookie seT-cookie SeT-cookie sET-cookie SET-cookie "+
	"set-Cookie Set-Cookie sEt-Cookie SEt-Cookie seT-Cookie SeT-Cookie sET-Cookie SET-Cookie "+
	"set-cOokie Set-cOokie sEt-cOokie SEt-cOokie seT-cOokie SeT-cOokie sET-cOokie SET-cOokie "+
	"set-COokie Set-COokie sEt-COokie SEt-COokie seT-COokie SeT-COokie sET-COokie SET-COokie "+
	"set-coOkie Set-coOkie sEt-coOkie SEt-coOkie seT-coOkie SeT-coOkie sET-coOkie SET-coOkie "+
	"set-CoOkie Set-CoOkie sEt-CoOkie SEt-CoOkie seT-CoOkie SeT-CoOkie sET-CoOkie SET-CoOkie "+
	"set-cOOkie Set-cOOkie sEt-cOOkie SEt-cOOkie seT-cOOkie SeT-cOOkie sET-cOOkie SET-cOOkie "+
	"set-COOkie Set-COOkie sEt-COOkie SEt-COOkie seT-COOkie SeT-COOkie sET-COOkie SET-COOkie", " ")

/* meta */
type meta struct {
	_mv                      bool
	Method, Path, Prefix, QS string
	Headers                  map[string]string
	Body                     []byte
}

func (meta *meta) api(ev _o) {
	ctx := ev["requestContext"].(_o)
	if ctx["stage"] != "$default" {
		meta.Prefix = _f("/%s", ctx["stage"])
	}
	if ctx["httpMethod"] != nil { // v1
		meta.Method = ctx["httpMethod"].(string)
	} else { // v2
		ctx = ctx["http"].(_o)
		meta.Method = ctx["method"].(string)
	}
	meta.Path = ctx["path"].(string)[len(meta.Prefix):]
}

func (meta *meta) alb(ev _o) {
	meta.Prefix = os.Getenv("PATH_PREFIX")
	meta.Method = ev["httpMethod"].(string)
	meta.Path = ev["path"].(string)[len(meta.Prefix):]
}

func _meta(ev _o) meta {
	meta := meta{Body: _body(ev["body"].(string), ev["isBase64Encoded"].(bool))}
	if ev["version"] == "1.0" { // api v1
		meta.api(ev)
		meta._mv = true
		meta.Headers = _headersMV(ev["multiValueHeaders"].(_o))
		meta.QS = _qsMV(ev["multiValueQueryStringParameters"].(_o), true)
		if !strings.HasSuffix(meta.Headers["host"], ".amazonaws.com") {
			evPath, metaPath := strings.Split(ev["path"].(string), "/"), strings.Split(meta.Path, "/")
			if len(evPath) > len(metaPath) {
				meta.Prefix = _f("/%s", evPath[1])
			}
		}
	} else if ev["version"] == "2.0" { // api v2
		meta.api(ev)
		meta._mv = false
		meta.Headers = _headers(ev["headers"].(_o))
		if cookies, ok := ev["cookies"].([]string); ok {
			meta.Headers["cookie"] = strings.Join(cookies, ";")
		}
		meta.QS = _f("?%s", ev["rawQueryString"])
		if !strings.HasSuffix(meta.Headers["host"], ".amazonaws.com") {
			meta.Prefix = os.Getenv("PATH_PREFIX")
		}
	} else if ev["headers"] != nil { // alb
		meta.alb(ev)
		meta._mv = false
		meta.Headers = _headers(ev["headers"].(_o))
		meta.QS = _qs(ev["queryStringParameters"].(_o))
	} else { // alb mv
		meta.alb(ev)
		meta._mv = true
		meta.Headers = _headersMV(ev["multiValueHeaders"].(_o))
		meta.QS = _qsMV(ev["multiValueQueryStringParameters"].(_o), false)
	}
	return meta
}

/* request */
func _req(meta meta) *http.Request {
	var headers []string
	for k, v := range meta.Headers {
		headers = append(headers, _f("%s: %s", k, v))
	}

	req, _ := http.ReadRequest(bufio.NewReader(bytes.NewReader(append(
		[]byte(_f(
			"%s %s%s HTTP/1.1\r\n%s\r\n\r\n",
			meta.Method, meta.Path, meta.QS,
			strings.Join(headers, "\r\n"),
		)),
		meta.Body...,
	))))
	req.RequestURI = _f("%s%s", meta.Prefix, req.RequestURI)
	req.RemoteAddr = strings.TrimSpace(meta.Headers["x-forwarded-for"][
		strings.LastIndex(meta.Headers["x-forwarded-for"], ",")+1:])

	return req
}

/* response */
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

/* handler */
// lambda.Start(lambdaGoHTTP.MakeHandler(http.DefaultServeMux))
func MakeHandler(handler http.Handler) func(ctx context.Context, ev _o) (_o, error) {
	return func(ctx context.Context, ev _o) (_o, error) {
		_debugEv(ev)
		meta := _meta(ev)
		_debugMeta(meta)
		w, req := httptest.NewRecorder(), _req(meta)
		_debugReq(req)
		_these[req] = &this{ctx, ev, meta}
		handler.ServeHTTP(w, req)
		delete(_these, req)
		rsp := _rsp(meta, w.Result())
		_debugRsp(rsp)
		return rsp, nil
	}
}

/* this related */
var _these = make(map[*http.Request]*this)

type this struct {
	Context context.Context
	Event   map[string]interface{}
	Meta    meta
}

// lambdaGoHTTP.GetThis(r).Event["httpMethod"]
func GetThis(r *http.Request) *this {
	return _these[r]
}
