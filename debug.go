package lambdahttp // import "marxus.github.io/go/lambdahttp"

import (
	"encoding/json"
	"fmt"
	"net/url"
)

var (
	_p    = fmt.Print
	DEBUG = false
)

func _json(v interface{}) string {
	bytes, _ := json.Marshal(v)
	return string(bytes)
}

func debugEv(ev _o) {
	if DEBUG {
		_p("\n\n//ev:\n\n", _json(ev))
	}
}

func debugMeta(m *meta) {
	if DEBUG {
		_p("\n\n//meta:\n\n", _json(m))
		parsedQS, _ := url.ParseQuery(m.QS)
		_p("\n\n//parsed_qs:\n\n", _json(parsedQS))
	}
}

func debugFound(found _o) {
	if DEBUG {
		_p("\n\n//found:\n\n", _json(found))
	}
}

func debugRsp(rsp _o) {
	if DEBUG {
		_p("\n\n//rsp:\n\n", _json(rsp))
	}
}
