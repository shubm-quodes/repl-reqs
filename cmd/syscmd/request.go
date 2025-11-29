package syscmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/config"
	"github.com/shubm-quodes/repl-reqs/log"
	"github.com/shubm-quodes/repl-reqs/network"
	"github.com/shubm-quodes/repl-reqs/util"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"

	"github.com/briandowns/spinner"
)

const (
	ActionCycleReq = "cycle_requests"
)

type ReqMgrAware interface {
	SetReqMgr(mgr *network.RequestManager)
}

type KeyValPair map[string]string
type ValidationSchema map[string]Validation

type ReqPropsSchema struct {
	QueryParams ValidationSchema `json:"queryParams"`
	UrlParams   ValidationSchema `json:"urlParams"`
	Body        ValidationSchema `json:"body"`
}

type BaseReqCmd struct {
	*cmd.BaseCmd
	Mgr *network.RequestManager
}

type ReqCmd struct {
	*BaseReqCmd
	*ReqPropsSchema
	*network.RequestDraft
}

type ReqCmdCfg struct {
	HttpMethod   string                `json:"httpMethod"`
	Cmd          string                `json:"cmd"`
	Url          string                `json:"url"`
	QueryParams  map[string]Validation `json:"queryParams"`
	UrlParams    map[string]Validation `json:"urlParams"`
	Body         map[string]Validation `json:"body"`
	RequestDraft *network.RequestDraft `json:"requestDraft"`
}

type CmdParams struct {
	URL   map[string]string
	Query map[string]string
	Body  map[string]any
}

var StopSpinnerChannel chan bool = make(chan bool)
var S *spinner.Spinner

type ReqSubCmd map[string]*ReqCmd

type Headers map[string]string
type PollCondition struct {
	KeySequence []string
	ExpectedVal string
}

func NewBaseReqCmd(name string) *BaseReqCmd {
	return &BaseReqCmd{
		BaseCmd: &cmd.BaseCmd{
			Name_: name,
		},
	}
}

func NewReqCmd(name string, mgr *network.RequestManager) *ReqCmd {
	return &ReqCmd{
		BaseReqCmd: NewBaseReqCmd(name),
		ReqPropsSchema: &ReqPropsSchema{
			QueryParams: make(ValidationSchema),
			UrlParams:   make(ValidationSchema),
			Body:        make(ValidationSchema),
		},
		RequestDraft: &network.RequestDraft{},
	}
}

func NewReqCfgFromCmd(rc *ReqCmd) *ReqCmdCfg {
	return &ReqCmdCfg{
		Cmd:          rc.GetFullyQualifiedName(),
		Url:          rc.Url,
		HttpMethod:   string(rc.Method),
		QueryParams:  rc.ReqPropsSchema.QueryParams,
		UrlParams:    rc.ReqPropsSchema.UrlParams,
		Body:         rc.ReqPropsSchema.Body,
		RequestDraft: rc.RequestDraft,
	}
}

func (brc *BaseReqCmd) SetReqMgr(mgr *network.RequestManager) {
	brc.Mgr = mgr
}

func (rc *ReqCmd) SetUrl(url string) *ReqCmd {
	rc.Url = url
	return rc
}

func (rc *ReqCmd) SetMethod(method network.HTTPMethod) *ReqCmd {
	rc.Method = method
	return rc
}

func (rc *ReqCmd) SetHeaders(headers KeyValPair) *ReqCmd {
	rc.Headers = headers
	return rc
}

func InitNetCmds(rawCfg config.RawCfg, hdlr *cmd.ReplCmdHandler) error {
	mgr := network.NewRequestManager(
		network.NewRequestTracker(),
		nil,
		strMapToHttpHeader(rawCfg.Commons.Headers),
	)
	if err := processRawReqCfg(rawCfg, hdlr, mgr); err != nil {
		return err
	}
	injectReqMgr(hdlr, mgr)
	registerListeners(hdlr, mgr)
	return nil
}

func registerListeners(hdlr *cmd.ReplCmdHandler, mgr *network.RequestManager) {
	hdlr.RegisterListener(0x10, ActionCycleReq, func() bool {
		ctxId := hdlr.GetDefaultCtx().Value(cmd.CmdCtxIdKey)
		if id, ok := ctxId.(string); ok {
			req, _ := mgr.CycleRequests(string(id))
			if req != nil {
				hdlr.SetPrompt(req.HttpRequest.URL.String(), "")
				hdlr.RefreshPrompt()
			}
		}
		return false
	})
}

func injectReqMgr(hdlr *cmd.ReplCmdHandler, mgr *network.RequestManager) {
	for _, c := range hdlr.GetCmdRegistry().GetAllCmds() {
		if c == nil {
			continue
		}
		if rmAware, ok := c.(ReqMgrAware); ok {
			rmAware.SetReqMgr(mgr)
		}
		if subCmds := c.GetSubCmds(); subCmds != nil {
			injectReqMgrIntoSubCmds(subCmds, mgr)
		}
	}
}

func injectReqMgrIntoSubCmds(reg map[string]cmd.Cmd, mgr *network.RequestManager) {
	for _, cmd := range reg {
		if rmAware, ok := cmd.(ReqMgrAware); ok {
			rmAware.SetReqMgr(mgr)
		}
		subCmds := cmd.GetSubCmds()
		if len(subCmds) > 0 {
			injectReqMgrIntoSubCmds(subCmds, mgr)
		}
	}
}

func (r *ReqCmd) UnmarshalJSON(data []byte) error {
	var rawProps struct {
		Cmd            string                     `json:"cmd"`
		Url            string                     `json:"url"`
		HttpMethod     network.HTTPMethod         `json:"httpMethod"`
		Headers        KeyValPair                 `json:"headers"`
		RawQueryParams map[string]json.RawMessage `json:"queryParams"`
		RawUrlParams   map[string]json.RawMessage `json:"urlParams"`
		RawBody        map[string]json.RawMessage `json:"body"`
	}

	if err := json.Unmarshal(data, &rawProps); err != nil {
		return err
	}

	r.Name_ = rawProps.Cmd // Temporarily assignment, gets overriden upon registration
	r.SetUrl(rawProps.Url).
		SetMethod(rawProps.HttpMethod).
		SetHeaders(rawProps.Headers)

	if err := r.initializeVlds(
		rawProps.RawUrlParams,
		rawProps.RawQueryParams,
		rawProps.RawBody,
	); err != nil {
		log.Warn("invalid validation specification for %s: %s", r.Name_, err.Error())
	}

	return nil
}

func (r *ReqCmd) MarshalJSON() ([]byte, error) {
	reqCfg := NewReqCfgFromCmd(r)
	return json.Marshal(reqCfg)
}

func processRawReqCfg(
	rawCfg config.RawCfg,
	hdlr *cmd.ReplCmdHandler,
	rMgr *network.RequestManager,
) error {
	for _, req := range rawCfg.RawRequests {
		reqCmd := NewReqCmd("", rMgr)
		if err := json.Unmarshal(req, &reqCmd); err != nil {
			return err
		} else {
			reqCmd.register(reqCmd.Name_, hdlr, rMgr)
		}
	}
	return nil
}

func strMapToHttpHeader(m map[string]string) http.Header {
	h := make(http.Header, len(m))
	for k, v := range m {
		h[k] = []string{v}
	}
	return h
}

func (rc *ReqCmd) register(
	cmdWithSubCmds string,
	hdlr cmd.CmdHandler,
	rMgr *network.RequestManager,
) {
	if strings.Trim(cmdWithSubCmds, " ") == "" {
		panic("cannot register request cmd without a command")
	}

	var command cmd.AsyncCmd
	segments := strings.Fields(cmdWithSubCmds)
	rootCmd := segments[0]
	cmdRegistry := hdlr.GetCmdRegistry()
	rc.Mgr = rMgr

	if len(segments) == 1 {
		rc.Name_ = rootCmd
		cmdRegistry.RegisterCmd(rc)
	}

	if existingCmd, exists := cmdRegistry.GetCmdByName(rootCmd); exists {
		if existingAsyncCmd, ok := existingCmd.(cmd.AsyncCmd); ok {
			command = existingAsyncCmd
		} else {
			panic(fmt.Sprintf("another non async command already registered with name '%s'", rootCmd))
		}
	} else {
		command = NewReqCmd(rootCmd, rMgr)
		cmdRegistry.RegisterCmd(command)
	}

	segments = segments[1:]
	remainingTkns, subCmd := command.WalkTillLastSubCmd(
		command.GetSubCmds(),
		util.StrArrToRune(segments),
	)
	for i, token := range remainingTkns {
		isLast := i == len(segments)-1
		if isLast {
			rc.Name_ = string(token)
			subCmd.AddSubCmd(rc)
		} else {
			subCmd = subCmd.AddSubCmd(NewReqCmd(string(token), rMgr))
		}
	}
}

func (rc *ReqCmd) initializeVlds(
	rawUrlParams, rawQueryParams, rawBody map[string]json.RawMessage,
) error {
	var err error
	if rc.UrlParams, err = constructValidationSchema(rawUrlParams); err != nil {
		return fmt.Errorf("failed to construct validation schema for url params %w", err)
	}

	if rc.ReqPropsSchema.QueryParams, err = constructValidationSchema(rawQueryParams); err != nil {
		return fmt.Errorf("failed to construct validation schema for query params %w", err)
	}

	return rc.constructBodyVldSchema(rawBody)
}

func (rc *ReqCmd) constructBodyVldSchema(rawBody map[string]json.RawMessage) error {
	var (
		bodySchema map[string]json.RawMessage
		err        error
	)

	if rawBodySchema, ok := rawBody["schema"]; !ok {
		log.Debug(
			"body schema not specified for req cmd '%s'. skipping validation loading",
			rc.Name(),
		)
		return nil
	} else {
		err = json.Unmarshal(rawBodySchema, &bodySchema)
		if err != nil {
			return err
		}
		if rc.ReqPropsSchema.Body, err = constructValidationSchema(bodySchema); err != nil {
			return err
		}
		return nil
	}
}

func (rc *ReqCmd) getCmdParams(tokens []string) (*CmdParams, error) {
	parsedParams, err := cmd.ParseCmdKeyValPairs(tokens)
	if err != nil {
		return nil, err
	}

	cmdParams := &CmdParams{
		URL:   make(map[string]string),
		Query: make(map[string]string),
		Body:  make(map[string]any),
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
		if err := validate(key, value, rc.ReqPropsSchema.UrlParams, cmdParams.URL); err != nil {
			return nil, err
		}
		if processedKeys[key] {
			continue
		}
		if err := validate(key, value, rc.ReqPropsSchema.QueryParams, cmdParams.Query); err != nil {
			return nil, err
		}
		if processedKeys[key] {
			continue
		}
		if err := validate(key, value, rc.ReqPropsSchema.Body, cmdParams.Body); err != nil {
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
	draft := rc.RequestDraft
	finalURL := draft.Url
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
	if len(cmdParams.Body) > 0 {
		bodyBytes, err := json.Marshal(cmdParams.Body)
		if err != nil {
			return nil, fmt.Errorf("error marshalling body: %w", err)
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(string(draft.Method), u.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}

	for key, value := range draft.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

func (rc *ReqCmd) extractTokenKey(token []rune) string {
	s := string(token)

	if s == "" {
		return ""
	}

	parts := strings.SplitN(s, "=", 2)
	return parts[0]
}

func (rc *ReqCmd) isTokenPreExisting(token []rune, remainingTkns [][]rune) bool {
	sugKey := rc.extractTokenKey(token)

	if sugKey == "" {
		return false
	}

	for _, rmTkn := range remainingTkns {
		if rc.extractTokenKey(rmTkn) == sugKey {
			return true
		}
	}

	return false
}

func (rc *ReqCmd) filterRedundantTokens(suggestions, remainingTkns [][]rune) [][]rune {
	var filteredSugg [][]rune

	for _, sug := range suggestions {
		if !rc.isTokenPreExisting(sug, remainingTkns) {
			filteredSugg = append(filteredSugg, sug)
		}
	}

	return filteredSugg
}

func (rc *ReqCmd) walkCommandTree(tokens [][]rune) (*ReqCmd, [][]rune) {
	remainingTkns, lastFoundCmd := cmd.Walk(rc, rc.SubCmds, tokens)

	if lastFoundCmd == nil && len(remainingTkns) > 0 {
		lastFoundCmd = rc
	}

	finalCmd, ok := lastFoundCmd.(*ReqCmd)
	if !ok || finalCmd == nil {
		return nil, nil
	}

	return finalCmd, remainingTkns
}

func (rc *ReqCmd) suggestParameters(
	finalCmd *ReqCmd,
	remainingTkns [][]rune,
) (suggestions [][]rune, offset int) {
	search := rc.getSearchQuery(remainingTkns)
	offset = len(search)

	suggestions = finalCmd.SuggestCmdParams(search)

	if len(suggestions) > 0 && len(remainingTkns) > 0 {
		suggestions = rc.filterRedundantTokens(suggestions, remainingTkns)
	}

	return
}

func (rc *ReqCmd) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	finalCmd, remainingTkns := rc.walkCommandTree(tokens)
	suggestions, offset := finalCmd.BaseCmd.GetSuggestions(remainingTkns)

	if len(suggestions) > 0 {
		return suggestions, offset
	}

	return rc.suggestParameters(finalCmd, remainingTkns)
}

func (rc *ReqCmd) getSearchQuery(remainingTkns [][]rune) []rune {
	if len(remainingTkns) == 0 {
		return nil
	}

	lastToken := string(remainingTkns[len(remainingTkns)-1])

	if parts := strings.SplitN(lastToken, "=", 2); len(parts) != 2 {
		return []rune(lastToken)
	}

	return nil
}

func (rc *ReqCmd) SuggestCmdParams(search []rune) (suggestions [][]rune) {
	if rc.RequestDraft == nil {
		return
	}
	schema := rc.ReqPropsSchema
	params := []ValidationSchema{schema.QueryParams, schema.UrlParams, schema.Body}
	criteria := &util.MatchCriteria[Validation]{
		Search:     string(search),
		SuffixWith: "=",
	}
	for _, p := range params {
		if p == nil {
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

func (rc *ReqCmd) ExecuteAsync(cmdCtx *cmd.CmdCtx) {
	tokens := cmdCtx.ExpandedTokens
	hdlr := rc.GetCmdHandler()
	taskUpdate := hdlr.GetUpdateChan()
	taskStatus := rc.GetTaskStatus()

	cmdParams, err := rc.getCmdParams(tokens)
	if err != nil {
		taskStatus.SetError(err)
		taskUpdate <- (*taskStatus)
		return
	}

	req, err := rc.buildRequest(cmdParams)
	if err != nil {
		taskStatus.SetError(err)
		taskUpdate <- (*taskStatus)
		return
	}

	rc.MakeRequest(req)
}

func (rc *ReqCmd) MakeRequest(req *http.Request) {
	taskStatus := rc.GetTaskStatus()
	taskUpdate := rc.GetCmdHandler().GetUpdateChan()
	_, netUpdate, err := rc.Mgr.MakeRequest(req)

	if err != nil {
		taskStatus.SetError(err)
		taskUpdate <- (*taskStatus)
		return
	}

	result := <-netUpdate
	if result.Err() == nil {
		rc.handleSuccessfulResponse(taskStatus, result)
	} else {
		taskStatus.SetError(result.Err())
		taskStatus.SetOutput(result.Err().Error())
	}
	taskStatus.SetDone(true)
	taskUpdate <- (*taskStatus)
}

func (rc *ReqCmd) readAndUnmarshalResponse(resp *http.Response, target map[string]any) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Restore it so it can be read again later
	resp.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

	err = json.Unmarshal(bodyBytes, &target)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return nil
}

func (rc *ReqCmd) handleSuccessfulResponse(taskStatus *cmd.TaskStatus, result network.Update) {
	respMap := make(map[string]any)

	err := rc.readAndUnmarshalResponse(result.Resp(), respMap)
	if err != nil {
		taskStatus.SetError(err)
		taskStatus.SetOutput(err.Error() + "\n\n" + result.Resp().Status)
		return
	}

	taskStatus.SetResult(result.Resp())
	taskStatus.SetOutput(getFromattedResp(respMap) + "\n" + result.Resp().Status)
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

func getFromattedResp(resp map[string]any) string {
	var jsonBuffer bytes.Buffer
	enc := json.NewEncoder(&jsonBuffer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(resp)

	lexer := lexers.Get("json")
	respStr := jsonBuffer.String()
	return highlightText(respStr, lexer)
}

func (rc *ReqCmd) PopulateSchemasFromDraft() {
	if rc.RequestDraft == nil {
		return
	}

	if rc.ReqPropsSchema == nil {
		rc.ReqPropsSchema = &ReqPropsSchema{
			QueryParams: make(ValidationSchema),
			Body:        make(ValidationSchema),
			UrlParams:   make(ValidationSchema),
		}
	}

	rc.populateQuerySchemaFromDraft()
	rc.ReqPropsSchema.Body = populateSchemaFromJSONString(rc.GetBody())
}

func (rc *ReqCmd) cleanup() {}

func (rc *ReqCmd) populateQuerySchemaFromDraft() {
	schema := rc.ReqPropsSchema
	schema.QueryParams = make(ValidationSchema)

	handlerFunc := func(key, value string) {
		if _, err := strconv.Atoi(value); err == nil {
			schema.QueryParams[key] = &IntValidations{Type: "int"}
		} else if _, err := strconv.ParseFloat(value, 64); err == nil {
			schema.QueryParams[key] = &FloatValidations{Type: "float"}
		} else {
			schema.QueryParams[key] = &StrValidations{Type: "string"}
		}
	}

	rc.IterateQueryParams(handlerFunc)
	schema.Body = populateSchemaFromJSONString(rc.RequestDraft.GetBody())
}

func inferTypeSchema(value any) Validation {
	if value == nil {
		return &StrValidations{Type: "string"}
	}

	switch v := value.(type) {
	case string:
		return &StrValidations{Type: "string"}
	case float64:
		if v == float64(int64(v)) {
			return &IntValidations{Type: "int"}
		}
		return &FloatValidations{Type: "float"}

	case map[string]any:
		objVld := &ObjValidation{Type: "object", fields: map[string]Validation{}}
		for key, subValue := range v {
			objVld.fields[key] = inferTypeSchema(subValue)
		}
		return objVld

	case []any:
		arrVld := &ArrValidation{Type: "array", arr: make([]Validation, len(v))}
		for idx, item := range v {
			if item != nil {
				arrVld.arr[idx] = inferTypeSchema(item)
				break
			}
		}

		return arrVld

	default:
		return &StrValidations{}
	}
}

func populateSchemaFromJSONString(jsonString string) ValidationSchema {
	schema := make(ValidationSchema)
	var bodyMap map[string]any

	if jsonString == "" {
		return schema
	}

	if err := json.Unmarshal([]byte(jsonString), &bodyMap); err != nil {
		fmt.Printf("Error unmarshalling JSON: %v\n", err)
		return schema
	}

	for key, value := range bodyMap {
		schema[key] = inferTypeSchema(value)
	}

	return schema
}

func loadConfig() (map[string]json.RawMessage, error) {
	raw, err := GetRawReqCfg()
	if err != nil {
		return nil, fmt.Errorf("failed to load config.json: %w", err)
	}

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config.json: %w", err)
	}
	return cfg, nil
}

func saveConfig(cfg map[string]json.RawMessage) error {
	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	path := config.GetAppCfg().CfgFilePath()
	if err := os.WriteFile(path, bytes, 0644); err != nil {
		return fmt.Errorf("failed to write config.json: %w", err)
	}

	return nil
}

func extractRequests(cfg map[string]json.RawMessage) ([]*ReqCmd, error) {
	rawReqCfg, ok := cfg["requests"]
	if !ok {
		return []*ReqCmd{}, nil
	}

	var reqs []json.RawMessage
	if err := json.Unmarshal(rawReqCfg, &reqs); err != nil {
		return nil, err
	}

	reqCmds := make([]*ReqCmd, len(reqs))
	for idx, r := range reqs {
		rc := NewReqCmd("", nil)
		if err := json.Unmarshal(r, rc); err != nil {
			return nil, err
		} else {
			reqCmds[idx] = rc
		}
	}

	return reqCmds, nil
}

func upsertRequest(requests []*ReqCmd, rc *ReqCmd) []*ReqCmd {
	fqCmd := rc.GetFullyQualifiedName()

	for i, r := range requests {
		if r.Name() == fqCmd {
			requests[i] = rc
			return requests
		}
	}

	return append(requests, rc)
}

func UpsertReqCfg(rc *ReqCmd) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	requests, err := extractRequests(cfg)
	if err != nil {
		return err
	}

	requests = upsertRequest(requests, rc)

	if rcmdBytes, err := json.MarshalIndent(requests, "", "  "); err != nil {
		return err
	} else {
		cfg["requests"] = rcmdBytes
	}

	return saveConfig(cfg)
}

func GetRawReqCfg() ([]byte, error) {
	cfgFilePath := config.GetAppCfg().CfgFilePath()
	return os.ReadFile(cfgFilePath)
}
