package network

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/shubm-quodes/repl-reqs/config"
	"github.com/shubm-quodes/repl-reqs/util"
)

type HTTPMethod string

const (
	GET     HTTPMethod = http.MethodGet
	HEAD    HTTPMethod = http.MethodHead
	POST    HTTPMethod = http.MethodPost
	PUT     HTTPMethod = http.MethodPut
	PATCH   HTTPMethod = http.MethodPatch
	DELETE  HTTPMethod = http.MethodDelete
	CONNECT HTTPMethod = http.MethodConnect
	OPTIONS HTTPMethod = http.MethodOptions
	TRACE   HTTPMethod = http.MethodTrace
)

type RequestDraft struct {
	id          string
	Url         string            `json:"url"         toml:"url"`
	Method      HTTPMethod        `json:"method"      toml:"method"`
	Headers     map[string]string `json:"headers"     toml:"headers"`
	QueryParams map[string]string `json:"queryParams" toml:"query_params"` // Different casing for TOML standard
	Payload     string            `json:"payload"     toml:"payload"`
}

type FuncQueryParamHandler func(key, val string)

func NewRequestDraft() *RequestDraft {
	return &RequestDraft{
		id: uuid.NewString(),
	}
}

func (rd *RequestDraft) GetId() string {
	return rd.id
}

func (rd *RequestDraft) GetUrl() string {
	return rd.Url
}

func (rd *RequestDraft) GetMethod() HTTPMethod {
	return rd.Method
}

func (rd *RequestDraft) GetHeader(key string) string {
	if rd.Headers != nil {
		return rd.Headers[key]
	}
	return ""
}

func (rd *RequestDraft) GetQueryParam(key string) string {
	if rd.QueryParams != nil {
		return rd.QueryParams[key]
	}
	return ""
}

func (rd *RequestDraft) GetPayload() string {
	return rd.Payload
}

func (rd *RequestDraft) SetUrl(url string) *RequestDraft {
	rd.Url = url
	return rd
}

func (rd *RequestDraft) SetMethod(method HTTPMethod) *RequestDraft {
	rd.Method = method
	return rd
}

func (rd *RequestDraft) SetHeader(key, val string) *RequestDraft {
	if rd.Headers == nil {
		rd.Headers = make(map[string]string)
	}

	rd.Headers[key] = val
	return rd
}

func (rd *RequestDraft) SetQueryParam(key, val string) *RequestDraft {
	if rd.QueryParams == nil {
		rd.QueryParams = make(map[string]string)
	}

	rd.QueryParams[key] = val
	return rd
}

func (rd *RequestDraft) SetPayload(payload string) *RequestDraft {
	rd.Payload = payload
	return rd
}

func (rd *RequestDraft) parseToHttpHeader() (http.Header, error) {
	result, err := rd.getExpandedKeyVals(rd.Headers)
	if err != nil {
		return nil, err
	}
	return http.Header(result), nil
}

func (rd *RequestDraft) parseToUrlVals() (url.Values, error) {
	result, err := rd.getExpandedKeyVals(rd.QueryParams)
	if err != nil {
		return nil, err
	}
	return url.Values(result), nil
}

func (rd *RequestDraft) getExpandedKeyVals(input map[string]string) (map[string][]string, error) {
	expandedKeyVals := make(map[string][]string, len(input))

	for k, v := range input {
		expandedVal, err := util.ReplaceStrPattern(
			v,
			config.VarPattern,
			config.GetEnvManager().GetActiveEnvVars(),
		)
		if err != nil {
			return nil, err
		}
		expandedKeyVals[k] = []string{expandedVal}
	}

	return expandedKeyVals, nil
}

func (d *RequestDraft) IterateQueryParams(handler FuncQueryParamHandler) {
	if d.QueryParams == nil {
		return
	}
	for key, value := range d.QueryParams {
		handler(key, value)
	}
}

func (rd *RequestDraft) Finalize() (*http.Request, error) {
	envMgr := config.GetEnvManager()
	lookups := envMgr.GetActiveEnvVars()

	if rd.Url == "" || rd.Method == "" {
		return nil, errors.New("cannot finalize request draft, url and method not set")
	}

	expandedUrl, err := util.ReplaceStrPattern(rd.Url, config.VarPattern, lookups)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(string(rd.Method), expandedUrl, nil)
	if err != nil {
		return nil, err
	}

	headers, err := rd.parseToHttpHeader()
	if err != nil {
		return nil, err
	}
	req.Header = headers

	query, err := rd.parseToUrlVals()
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = query.Encode()
	parsedPayload, err := util.ReplaceStrPattern(rd.Payload, config.VarPattern, lookups)
	if err != nil {
		return nil, err
	}

	if len(parsedPayload) > 0 {
		req.Body = io.NopCloser(strings.NewReader(parsedPayload))
		req.ContentLength = int64(len(parsedPayload))
	}

	return req, nil
}

func (r *RequestDraft) GetKey() string {
	return r.id
}

func (r *RequestDraft) EditAsToml() error {
	return util.EditToml(r, config.GetAppCfg().GetDefaultEditor())
}

func IsValidHttpVerb(verb HTTPMethod) bool {
	switch verb {
	case GET, HEAD, POST, PUT, PATCH, DELETE, CONNECT, OPTIONS, TRACE:
		return true
	default:
		return false
	}
}
