package network

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/shubm-quodes/repl-reqs/log"
	"github.com/shubm-quodes/repl-reqs/util"
)

var PollConditionRegex = regexp.MustCompile(
	`^\$(header|body|status)([^=]*)=(.*)$`,
)

type Condition interface {
	Evaluate(resp *http.Response) bool
	IsMandatory() bool
}

type BaseCondition struct{}

type StatusCondition struct {
	*BaseCondition
	Expected string
}

type HeaderCondition struct {
	*BaseCondition
	Key      string
	Expected string
}

type BodyCondition struct {
	*BaseCondition
	Path     string
	Expected string
}

func NewCondition(raw string) (Condition, error) {
	matches := PollConditionRegex.FindStringSubmatch(raw)
	if matches == nil {
		return nil, fmt.Errorf("invalid condition format")
	}

	kind, path, value := matches[1], matches[2], matches[3]

	if len(path) < 2 {
		return nil, fmt.Errorf("invalid path '%s' for condition", path)
	}

	path = path[1:]

	switch kind {
	case "status":
		return &StatusCondition{Expected: value}, nil
	case "header":
		return &HeaderCondition{Key: path, Expected: value}, nil
	case "body":
		return &BodyCondition{Path: path, Expected: value}, nil
	default:
		return nil, fmt.Errorf("unknown condition type: %s", kind)
	}
}

func (b *BaseCondition) IsMandatory() bool {
	return false
}

func (c *StatusCondition) Evaluate(resp *http.Response) bool {
	return strconv.Itoa(resp.StatusCode) == c.Expected
}

func (c *HeaderCondition) Evaluate(resp *http.Response) bool {
	key := strings.TrimPrefix(c.Key, ".")
	return resp.Header.Get(key) == c.Expected
}

/*
* Unmarshal body as per the content type.
* currently xml & json are supported.. what else???
* Extract the value specified by the condition.
* As per the type of the value, perform asertion and compare the val.
 */
func (c *BodyCondition) Evaluate(resp *http.Response) bool {
	body, err := c.getUnmarshalledBody(resp)
	if err != nil {
		return false
	}

	return c.evaluate(body)
}

func (c *BodyCondition) evaluate(body any) bool {
	val, err := util.ExtractVal(body, c.Path)
	if err != nil { // We never know.. maybe the property will appear in upcoming responses..
		log.Debug("failed to extract value from response body for condition's path '%s'", c.Path)
		return false
	}

	return util.IsStrEqualToAny(c.Expected, val)
}

func (c *BodyCondition) getUnmarshalledBody(resp *http.Response) (any, error) {
	cType := resp.Header.Get("Content-Type")

	bodyBytes, err := util.ReadAndResetIoCloser(&resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var body any

	switch {
	case strings.Contains(cType, "application/json"):
		err = json.Unmarshal(bodyBytes, &body)
	case strings.Contains(cType, "xml"):
		err = xml.Unmarshal(bodyBytes, &body)
	default:
		return nil, fmt.Errorf("unsupported Content-Type: %s", cType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", cType, err)
	}

	return body, nil
}
