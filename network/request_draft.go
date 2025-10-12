package network

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/nodding-noddy/repl-reqs/config"
	"github.com/nodding-noddy/repl-reqs/util"
)

type RequestDraft struct {
	id          string
	url         string
	method      string
	headers     map[string]string
	queryParams map[string]string
	payload     string
}

func NewRequestDraft() *RequestDraft {
	return &RequestDraft{
		id: uuid.NewString(),
	}
}

func (rd *RequestDraft) GetId() string {
	return rd.id
}

func (rd *RequestDraft) GetUrl() string {
	return rd.url
}

func (rd *RequestDraft) GetMethod() string {
	return rd.method
}

func (rd *RequestDraft) GetHeader(key string) string {
	if rd.headers != nil {
		return rd.headers[key]
	}
	return ""
}

func (rd *RequestDraft) GetQueryParam(key string) string {
	if rd.queryParams != nil {
		return rd.queryParams[key]
	}
	return ""
}

func (rd *RequestDraft) GetPayload() string {
	return rd.payload
}

func (rd *RequestDraft) SetUrl(url string) *RequestDraft {
	rd.url = url
	return rd
}

func (rd *RequestDraft) SetMethod(method string) *RequestDraft {
	rd.method = method
	return rd
}

func (rd *RequestDraft) SetHeader(key, val string) *RequestDraft {
	if rd.headers == nil {
		rd.headers = make(map[string]string)
	}

	rd.headers[key] = val
	return rd
}

func (rd *RequestDraft) SetQueryParam(key, val string) *RequestDraft {
	if rd.queryParams == nil {
		rd.queryParams = make(map[string]string)
	}

	rd.queryParams[key] = val
	return rd
}

func (rd *RequestDraft) SetPayload(payload string) *RequestDraft {
	rd.payload = payload
	return rd
}

func (rd *RequestDraft) parseToHttpHeader() (http.Header, error) {
	result, err := rd.getExpandedKeyVals(rd.headers)
	if err != nil {
		return nil, err
	}
	return http.Header(result), nil
}

func (rd *RequestDraft) parseToUrlVals() (url.Values, error) {
	result, err := rd.getExpandedKeyVals(rd.queryParams)
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

func (rd *RequestDraft) Finalize() (*http.Request, error) {
	envMgr := config.GetEnvManager()
	lookups := envMgr.GetActiveEnvVars()

	if rd.url == "" || rd.method == "" {
		return nil, errors.New("cannot finalize request draft, url and method not set")
	}

	expandedUrl, err := util.ReplaceStrPattern(rd.url, config.VarPattern, lookups)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(rd.method, expandedUrl, nil)
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
	parsedPayload, err := util.ReplaceStrPattern(rd.payload, config.VarPattern, lookups)
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
