package lambdahttp // import "marxus.github.io/go/lambdahttp"

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
	"strconv"
	"strings"
)

var (
	_f         = fmt.Sprintf
	_setCookie = strings.Split(""+
		"set-cookie Set-cookie sEt-cookie SEt-cookie seT-cookie SeT-cookie sET-cookie SET-cookie "+
		"set-Cookie Set-Cookie sEt-Cookie SEt-Cookie seT-Cookie SeT-Cookie sET-Cookie SET-Cookie "+
		"set-cOokie Set-cOokie sEt-cOokie SEt-cOokie seT-cOokie SeT-cOokie sET-cOokie SET-cOokie "+
		"set-COokie Set-COokie sEt-COokie SEt-COokie seT-COokie SeT-COokie sET-COokie SET-COokie "+
		"set-coOkie Set-coOkie sEt-coOkie SEt-coOkie seT-coOkie SeT-coOkie sET-coOkie SET-coOkie "+
		"set-CoOkie Set-CoOkie sEt-CoOkie SEt-CoOkie seT-CoOkie SeT-CoOkie sET-CoOkie SET-CoOkie "+
		"set-cOOkie Set-cOOkie sEt-cOOkie SEt-cOOkie seT-cOOkie SeT-cOOkie sET-cOOkie SET-cOOkie "+
		"set-COOkie Set-COOkie sEt-COOkie SEt-COOkie seT-COOkie SeT-COOkie sET-COOkie SET-COOkie", " ")
)

type (
	_o               = map[string]interface{}
	ResponseRecorder struct{ *httptest.ResponseRecorder }
)

func (*ResponseRecorder) CloseNotify() <-chan bool { return nil }

/* request parsing related */
func parseBody(_body string, decode bool) []byte {
	body := []byte(_body)
	if decode {
		n, _ := base64.StdEncoding.Decode(body, body)
		body = body[:n]
	}
	return body
}

func parseHeaders(_headers _o) map[string]string {
	headers := make(map[string]string)
	for k, v := range _headers {
		headers[k] = v.(string)
	}
	return headers
}

func parseHeadersMV(_headers _o) map[string]string {
	headers := make(map[string]string)
	for k, vs := range _headers {
		for _, v := range vs.([]interface{}) {
			headers[k] += _f("%s,", v)
		}
		headers[k] = headers[k][:len(headers[k])-1]
	}
	return headers
}

func extractHeaderV(v string) string {
	return strings.TrimSpace(v[strings.LastIndex(v, ",")+1:])
}

func parseQS(_qs _o) string {
	var qs []string
	for k, v := range _qs {
		qs = append(qs, _f("%s=%s", k, v))
	}
	return strings.Join(qs, "&")
}

func parseQSMV(_qs _o, escape bool) string {
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
	return strings.Join(qs, "&")
}

/* meta */
type meta struct {
	_mv                  bool
	RemoteAddr           string
	Method, Scheme, Host string
	Port                 int
	Prefix, Path, QS     string
	Headers              map[string]string
	Body                 []byte
}

func (m *meta) api(ev _o) {
	ctx := ev["requestContext"].(_o)

	if ctx["stage"] != "$default" {
		m.Prefix = _f("/%s", ctx["stage"])
	}

	if ctx["httpMethod"] != nil { // v1
		m.Method = ctx["httpMethod"].(string)
	} else { // v2
		ctx = ctx["http"].(_o)
		m.Method = ctx["method"].(string)
	}

	m.Path = ctx["path"].(string)[len(m.Prefix):]
}

func (m *meta) alb(ev _o) {
	m.Prefix = os.Getenv("PATH_PREFIX")
	m.Method = ev["httpMethod"].(string)
	m.Path = ev["path"].(string)[len(m.Prefix):]
}

func getMetaFor(ev _o) *meta {
	m := &meta{Body: parseBody(ev["body"].(string), ev["isBase64Encoded"].(bool))}

	if ev["version"] == "1.0" { // api v1
		m.api(ev)
		m._mv = true
		m.Headers = parseHeadersMV(ev["multiValueHeaders"].(_o))
		m.QS = parseQSMV(ev["multiValueQueryStringParameters"].(_o), true)
		if !strings.HasSuffix(m.Headers["host"], ".amazonaws.com") {
			evPath, metaPath := strings.Split(ev["path"].(string), "/"), strings.Split(m.Path, "/")
			if len(evPath) > len(metaPath) {
				m.Prefix = _f("/%s", evPath[1])
			}
		}

	} else if ev["version"] == "2.0" { // api v2
		m.api(ev)
		m._mv = false
		m.Headers = parseHeaders(ev["headers"].(_o))
		if cookies, ok := ev["cookies"].([]string); ok {
			m.Headers["cookie"] = strings.Join(cookies, ";")
		}
		m.QS = ev["rawQueryString"].(string)
		if !strings.HasSuffix(m.Headers["host"], ".amazonaws.com") {
			m.Prefix = os.Getenv("PATH_PREFIX")
		}

	} else if ev["headers"] != nil { // alb
		m.alb(ev)
		m._mv = false
		m.Headers = parseHeaders(ev["headers"].(_o))
		m.QS = parseQS(ev["queryStringParameters"].(_o))

	} else { // alb mv
		m.alb(ev)
		m._mv = true
		m.Headers = parseHeadersMV(ev["multiValueHeaders"].(_o))
		m.QS = parseQSMV(ev["multiValueQueryStringParameters"].(_o), false)
	}

	m.QS = strings.ReplaceAll(m.QS, " ", "%20")
	m.RemoteAddr = extractHeaderV(m.Headers["x-forwarded-for"])
	m.Scheme = extractHeaderV(m.Headers["x-forwarded-proto"])
	m.Host = m.Headers["host"]
	m.Port, _ = strconv.Atoi(extractHeaderV(m.Headers["x-forwarded-port"]))
	return m
}

/* request */
func getReqFor(m *meta) *http.Request {
	var headers []string
	for k, v := range m.Headers {
		headers = append(headers, _f("%s: %s", k, v))
	}

	req, _ := http.ReadRequest(bufio.NewReader(bytes.NewReader(append(
		[]byte(_f(
			"%s %s?%s HTTP/1.1\r\n%s\r\n\r\n",
			m.Method, m.Path, m.QS,
			strings.Join(headers, "\r\n"),
		)),
		m.Body...,
	))))
	req.RemoteAddr = m.RemoteAddr
	req.URL.Scheme = m.Scheme
	req.URL.Host = m.Host
	return req
}

/* response */
func returnRsp(m *meta, _rsp *http.Response) _o {
	body, _ := io.ReadAll(_rsp.Body)
	rsp := _o{
		"statusCode":        _rsp.StatusCode,
		"statusDescription": _rsp.Status,
		"body":              base64.StdEncoding.EncodeToString(body),
		"isBase64Encoded":   true,
	}

	if !m._mv {
		headers := make(_o)
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

/* found */
func returnFound(m *meta) _o {
	rsp, location := _o{
		"statusCode":        302,
		"statusDescription": "302 Found",
	}, _f("%s/?%s", m.Prefix, m.QS)

	if !m._mv {
		rsp["headers"] = _o{"Location": location}
	} else {
		rsp["multiValueHeaders"] = _o{"Location": []string{location}}
	}

	return rsp
}

/* handler */
// lambda.Start(lambdahttp.MakeHandler(http.DefaultServeMux))
func MakeHandler(handler http.Handler) func(ctx context.Context, ev _o) (_o, error) {
	return func(ctx context.Context, ev _o) (_o, error) {
		debugEv(ev)
		m := getMetaFor(ev)
		debugMeta(m)
		if m.Path == "" {
			found := returnFound(m)
			debugFound(found)
			return found, nil
		}
		w, req := &ResponseRecorder{httptest.NewRecorder()}, getReqFor(m)
		these[req] = &this{ctx, ev, m}
		handler.ServeHTTP(w, req)
		delete(these, req)
		rsp := returnRsp(m, w.Result())
		debugRsp(rsp)
		return rsp, nil
	}
}

/* this related */
var these = make(map[*http.Request]*this)

type this struct {
	Context context.Context
	Event   map[string]interface{}
	Meta    *meta
}

// lambdahttp.GetThis(r).Event["httpMethod"]
func GetThis(r *http.Request) *this {
	return these[r]
}
