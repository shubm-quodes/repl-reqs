package syscmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nodding-noddy/repl-reqs/cmd"
	"github.com/nodding-noddy/repl-reqs/config"
	"github.com/nodding-noddy/repl-reqs/log"
	"github.com/nodding-noddy/repl-reqs/util"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"

	"github.com/briandowns/spinner"
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

type KeyValPair map[string]string
type ValidationSchema map[string]Validation

type ReqProps struct {
	Url         string           `json:"url"`
	HttpMethod  HTTPMethod       `json:"httpMethod"`
	Headers     KeyValPair       `json:"headers"`
	QueryParams ValidationSchema `json:"queryParams"`
	UrlParams   ValidationSchema `json:"urlParams"`
	Payload     ValidationSchema `json:"payload"`
}

type ReqCmd struct {
	mgr *RequestManager
	*cmd.BaseCmd
	*ReqProps
}

type ReqData struct {
	queryParams KeyValPair
	Headers     KeyValPair
	Payload     map[string]any
}

type RequestManager struct {
	Tracker       *RequestTracker
	Client        *http.Client
	CommonHeaders http.Header
}

type CmdParams struct {
	URL     map[string]string
	Query   map[string]string
	Payload map[string]any
}

var ProccessingReqs = make([]*Request, 1)

var StopSpinnerChannel chan bool = make(chan bool)
var S *spinner.Spinner

type ReqSubCmd map[string]*ReqCmd

type Headers map[string]string
type PollCondition struct {
	KeySequence []string
	ExpectedVal string
}

type Req struct {
}

type Request struct {
	ID          string
	Command     *ReqCmd
	HttpRequest *http.Request
}

var commonHeaders = make(KeyValPair)

func NewRequestManager(tracker *RequestTracker, commonHeaders http.Header) *RequestManager {
	return &RequestManager{
		Tracker:       tracker,
		Client:        &http.Client{},
		CommonHeaders: commonHeaders,
	}
}

func (rm *RequestManager) MakeRequest(req *http.Request) (string, <-chan Update, error) {
	reqID := uuid.New().String()
	done := make(Done)

	trackerReq := &TrackerRequest{
		Request: &Request{
			ID:          reqID,
			HttpRequest: req,
		},
		Status: StatusProcessing,
		Done:   done,
	}

	rm.Tracker.AddRequest(trackerReq)
	update := Update{
		reqId: reqID,
	}

	updateChan := make(chan Update)
	go func(rm *RequestManager, id string, r *http.Request) {
		defer close(done)

		start := time.Now()
		resp, err := rm.Client.Do(r)
		if err != nil {
			trackerReq.Status = StatusError
			return
		}

		requestTime := time.Since(start)

		update.resp = resp
		rm.Tracker.updates <- update
		updateChan <- update

		trackerReq.RequestTime = requestTime
	}(rm, reqID, req)

	return reqID, updateChan, nil
}

func ParseRawReqs(rawCfg config.RawCfg, hdlr *cmd.CmdHandler) error {
	util.CopyMap(commonHeaders, rawCfg.Commons.Headers)
	for _, req := range rawCfg.RawRequests {
		var rawProps struct {
			Cmd            string          `json:"cmd"`
			Url            string          `json:"url"`
			HttpMethod     HTTPMethod      `json:"httpMethod"`
			Headers        KeyValPair      `json:"headers"`
			RawQueryParams json.RawMessage `json:"queryParams"`
			RawUrlParams   json.RawMessage `json:"urlParams"`
			RawPayload     json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(req, &rawProps); err != nil {
			return err
		}
		reqCmd := &ReqCmd{
			BaseCmd: &cmd.BaseCmd{
				Name_: "", //IMP: Since there could be multiple segments.
			},
			ReqProps: &ReqProps{
				HttpMethod: rawProps.HttpMethod,
				Url:        rawProps.Url,
				Headers:    rawProps.Headers,
			},
		}
		reqCmd.initializeVlds(
			rawProps.RawUrlParams,
			rawProps.RawQueryParams,
			rawProps.RawPayload,
		)
		reqCmd.register(rawProps.Cmd, hdlr)
	}
	return nil
}

func isValidHttpVerb(verb HTTPMethod) bool {
	switch verb {
	case GET, HEAD, POST, PUT, PATCH, DELETE, CONNECT, OPTIONS, TRACE:
		return true
	default:
		return false
	}
}

func (r *ReqCmd) register(cmdWithSubCmds string, hdlr *cmd.CmdHandler) {
	if strings.Trim(cmdWithSubCmds, " ") == "" {
		panic("cannot register request cmd without a command")
	}

	var command cmd.AsyncCmd
	segments := strings.Fields(cmdWithSubCmds)
	rootCmd := segments[0]
	cmdRegistry := hdlr.GetCmdRegistry()
	rMgr := NewRequestManager(NewRequestTracker(), nil)
	r.mgr = rMgr

	if len(segments) == 1 {
		r.Name_ = rootCmd
		cmdRegistry.RegisterCmd(r)
	}

	if existingCmd, exists := cmdRegistry.GetCmdByName(rootCmd); exists {
		if existingAsyncCmd, ok := existingCmd.(cmd.AsyncCmd); ok {
			command = existingAsyncCmd
		} else {
			panic(fmt.Sprintf("another non async command already registered with name '%s'", rootCmd))
		}
	} else {
		command = &ReqCmd{
			BaseCmd: &cmd.BaseCmd{
				Name_: rootCmd,
			},
			mgr: rMgr,
		}
		cmdRegistry.RegisterCmd(command)
	}

	segments = segments[1:]
	remainingTkns, subCmd := command.WalkTillLastSubCmd(util.StrArrToRune(segments))
	for i, token := range remainingTkns {
		isLast := i == len(segments)-1
		if isLast {
			r.Name_ = string(token)
			subCmd.AddSubCmd(r)
		} else {
			subCmd = subCmd.AddSubCmd(&ReqCmd{
				BaseCmd: &cmd.BaseCmd{
					Name_: string(token),
				},
				mgr: rMgr,
			})
		}
	}
}

func (rc *ReqCmd) initializeVlds(
	rawUrlParams, rawQueryParams, rawPayload json.RawMessage,
) {
	rc.UrlParams.initialize(rawUrlParams)
	rc.QueryParams.initialize(rawQueryParams)
	rc.Payload.initialize(rawPayload)
}

func (rc *ReqCmd) getCmdParams(tokens []string) (*CmdParams, error) {
	parsedParams, err := cmd.ParseCmdKeyValPairs(tokens)
	if err != nil {
		return nil, err
	}

	cmdParams := &CmdParams{
		URL:     make(map[string]string),
		Query:   make(map[string]string),
		Payload: make(map[string]any),
	}

	processedKeys := make(map[string]bool)

	validate := func(key string, value string, schema map[string]Validation, destMap any) error {
		if valSchema, ok := schema[key]; ok {
			validatedValue, err := valSchema.validate(value)
			if err != nil {
				return fmt.Errorf("validation failed for parameter '%s': %w", key, err)
			}
			switch m := destMap.(type) {
			case map[string]string:
				m[key] = fmt.Sprintf("%v", validatedValue)
			case map[string]any:
				m[key] = validatedValue
			}
			processedKeys[key] = true
		}
		return nil
	}

	for key, value := range parsedParams {
		if err := validate(key, value, rc.ReqProps.UrlParams, cmdParams.URL); err != nil {
			return nil, err
		}
		if processedKeys[key] {
			continue
		}
		if err := validate(key, value, rc.ReqProps.QueryParams, cmdParams.Query); err != nil {
			return nil, err
		}
		if processedKeys[key] {
			continue
		}
		if err := validate(key, value, rc.ReqProps.Payload, cmdParams.Payload); err != nil {
			return nil, err
		}
		if processedKeys[key] {
			continue
		}

		return nil, fmt.Errorf("unrecognized parameter '%s'", key)
	}

	return cmdParams, nil
}

func (rc *ReqCmd) buildRequest(cmdParams *CmdParams) (*http.Request, error) {
	finalURL := rc.ReqProps.Url
	for key, value := range cmdParams.URL {
		finalURL = strings.Replace(finalURL, ":"+key, value, 1)
	}
	u, err := url.Parse(finalURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}

	q := u.Query()
	for key, value := range cmdParams.Query {
		q.Add(key, value)
	}
	u.RawQuery = q.Encode()

	var reqBody io.Reader
	if len(cmdParams.Payload) > 0 {
		payloadBytes, err := json.Marshal(cmdParams.Payload)
		if err != nil {
			return nil, fmt.Errorf("error marshalling payload: %w", err)
		}
		reqBody = bytes.NewReader(payloadBytes)
	}

	req, err := http.NewRequest(string(rc.HttpMethod), u.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}

	for key, value := range rc.ReqProps.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

func (rc *ReqCmd) GetSuggestions(tokens [][]rune) (suggestions [][]rune, offset int) {
	remainingTkns, lastFoundCmd := cmd.Walk(rc, tokens)
	if lastFoundCmd == nil && len(remainingTkns) > 0 {
		lastFoundCmd = rc
	}

	if lastFoundCmd == nil {
		return
	}

	rc, ok := lastFoundCmd.(*ReqCmd)
	if !ok {
		return
	}

	suggestions, offset = rc.BaseCmd.GetSuggestions(remainingTkns)
	if len(suggestions) > 0 {
		return
	}

	var search []rune
	// if len(remainingTkns) == 0 {
	// 	search = []rune{}
	// } else {
	if len(remainingTkns) > 0 {
		lastToken := string(remainingTkns[len(remainingTkns)-1])
		if parts := strings.SplitN(lastToken, "=", 2); len(parts) != 2 {
			search = []rune(lastToken)
		}
	}

	offset = len(search)
	suggestions = rc.SuggestCmdParams(search)
	return
}

func (rc *ReqCmd) SuggestCmdParams(search []rune) (suggestions [][]rune) {
	params := []ValidationSchema{rc.QueryParams, rc.UrlParams, rc.Payload}
	criteria := &util.MatchCriteria[Validation]{
		Search:     string(search),
		SuffixWith: "=",
	}
	for _, p := range params {
		if p == nil {
			fmt.Println("is nill")
			continue
		}
		criteria.M = p
		matches := util.GetMatchingMapKeysAsRunes(criteria)
		if len(matches) > 0 {
			suggestions = append(suggestions, matches...)
		}
	}
	return
}

func (rc *ReqCmd) ExecuteAsync(cmdContext *cmd.CmdContext) {
	tokens := cmdContext.ExpandedTokens
	hdlr := rc.GetCmdHandler()
	uChan := hdlr.GetUpdateChan()
	t := rc.GetTaskStatus()

	cmdParams, err := rc.getCmdParams(tokens)
	if err != nil {
		t.SetError(err)
		uChan <- (*t)
		return
	}

	req, err := rc.buildRequest(cmdParams)
	if err != nil {
		t.SetError(err)
		uChan <- (*t)
		return
	}

	reqId, updateChan, err := rc.mgr.MakeRequest(req)
	if err != nil {
		t.SetError(err)
		uChan <- (*t)
		return
	}

	up := <-updateChan
	defer up.resp.Body.Close()

	resp := make(map[string]any)
	respBytes, err := io.ReadAll(up.resp.Body)
	if err != nil {
		fmt.Println("failed to read response body")
		log.Debug("failed to read response body", err.Error())
		return
	}
	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		fmt.Println("failed to process response content", err, string(respBytes))
		log.Debug("failed to unmarshal response body", err.Error())
		return
	}
	fmt.Printf("successfully completed request with id '%s'\n", reqId)

	t.SetDone(true)
	t.SetResult(up.resp)
	t.SetOutput(getFromattedResp(resp) + "\n" + up.resp.Status)
	uChan <- (*t)
}

func initialiseSpinner() {
	S = spinner.New(spinner.CharSets[43], 100*time.Millisecond)
	S.Start()
	go func() {
		<-StopSpinnerChannel
		if S.Active() {
			S.Stop()
		}
	}()
}

func rgbToAnsiEscapeCode(r, g, b uint8) string {
	ansiColor := 16 + (r/51)*36 + (g/51)*6 + (b / 51)
	return fmt.Sprintf("\033[38;5;%dm", ansiColor)
}

func highlightText(input string, lexer chroma.Lexer) string {
	iterator, err := lexer.Tokenise(nil, input)
	if err != nil {
		fmt.Println("Error tokenizing inputs:", err)
		return input
	}

	var buf bytes.Buffer
	style := styles.Get("monokai")
	tokens := iterator.Tokens()

	for _, token := range tokens {
		entry := style.Get(token.Type)
		r, g, b := entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue()
		ansiEscape := rgbToAnsiEscapeCode(r, g, b)
		buf.WriteString(fmt.Sprintf("%s%s\033[0m", ansiEscape, token.Value))
	}
	return buf.String()
}

func getFromattedResp(resp map[string]interface{}) string {
	var jsonBuffer bytes.Buffer
	enc := json.NewEncoder(&jsonBuffer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(resp)

	lexer := lexers.Get("json")
	respStr := jsonBuffer.String()
	return highlightText(respStr, lexer)
}

func (vld *ValidationSchema) initialize(rawVlds json.RawMessage) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(rawVlds, &rawMap); err != nil {
		log.Error(`Failed to initialize params: %s`, err.Error())
		os.Exit(1)
	}

	if *vld == nil {
		(*vld) = ValidationSchema{}
	}

	for paramName, val := range rawMap {
		var vldType struct {
			Type string `json:"type"`
		}

		if err := json.Unmarshal(val, &vldType); err != nil {
			log.Error(
				`Failed to initialize validations for "%s":`,
				paramName,
				err.Error(),
			)
			os.Exit(1)
		}

		if v, err := getValidation(vldType.Type); err != nil {
			log.Error(err.Error())
			os.Exit(1)
		} else {
			(*vld)[paramName] = v
		}
	}
}

func getValidation(paramType string) (Validation, error) {
	switch paramType {
	case "int":
		return &IntValidations{}, nil
	case "float":
		return &FloatValidations{}, nil
	case "string":
		return &StrValidations{}, nil
	default:
		return nil, fmt.Errorf(`invalid parameter type "%s"`, paramType)
	}
}

func (req *ReqCmd) cleanup() {
}
