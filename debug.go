package lambdaGoHTTP

import (
	"encoding/json"
	"fmt"
	"net/http"
)

var DEBUG = false

func _json(v interface{}) string {
	bytes, _ := json.Marshal(v)
	return string(bytes)
}

func _debugEv(ev _o) {
	if DEBUG {
		fmt.Print("\n\n//ev:\n\n", _json(ev))
	}
}

func _debugMeta(meta meta) {
	if DEBUG {
		fmt.Print("\n\n//meta:\n\n", _json(meta))
	}
}

func _debugReq(req *http.Request) {
	if DEBUG {
		req.ParseForm()
		fmt.Print("\n\n//req:\n\n", _json(map[string]interface{}{
			"Form":       req.Form,
			"Host":       req.Host,
			"RemoteAddr": req.RemoteAddr,
			"RequestURI": req.RequestURI,
			"URL":        req.URL,
		}))
	}
}

func _debugRsp(rsp _o) {
	if DEBUG {
		fmt.Print("\n\n//rsp:\n\n", _json(rsp))
	}
}
