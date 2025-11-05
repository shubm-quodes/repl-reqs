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
	Payload     ValidationSchema `json:"payload"`
}

type ReqProps struct {
	Url        string             `json:"url"`
	HttpMethod network.HTTPMethod `json:"httpMethod"`
	Headers    KeyValPair         `json:"headers"`
	*ReqPropsSchema
	*network.RequestDraft
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

type ReqData struct {
	queryParams KeyValPair
	Headers     KeyValPair
	Payload     map[string]any
}

type CmdParams struct {
	URL     map[string]string
	Query   map[string]string
	Payload map[string]any
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
			Payload:     make(ValidationSchema),
		},
		RequestDraft: &network.RequestDraft{},
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

func InitNetCmds(rawCfg config.RawCfg, hdlr *cmd.CmdHandler) error {
	mgr := network.NewRequestManager(
		network.NewRequestTracker(),
		nil,
		strMapToHttpHeader(rawCfg.Commons.Headers),
	)
	if err := parseRawReqCfg(rawCfg, hdlr, mgr); err != nil {
		return err
	}
	injectReqMgr(hdlr, mgr)
	registerListeners(hdlr, mgr)
	return nil
}

func registerListeners(hdlr *cmd.CmdHandler, mgr *network.RequestManager) {
	hdlr.RegisterListener(0x10, ActionCycleReq, func() bool {
		ctxId := hdlr.GetDefaultCtx().Value(cmd.CmdCtxIdKey)
		if id, ok := ctxId.(string); ok {
			req, _ := mgr.CycleRequests(string(id))
			// currDraft := mgr.PeakRequestDraft(string(id))
			if req != nil {
				hdlr.SetPrompt(req.HttpRequest.URL.String(), "")
				hdlr.RefreshPrompt()
			}
		}
		return false
	})
}

func injectReqMgr(hdlr *cmd.CmdHandler, mgr *network.RequestManager) {
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

func parseRawReqCfg(
	rawCfg config.RawCfg,
	hdlr *cmd.CmdHandler,
	rMgr *network.RequestManager,
) error {
	for _, req := range rawCfg.RawRequests {
		var rawProps struct {
			Cmd            string             `json:"cmd"`
			Url            string             `json:"url"`
			HttpMethod     network.HTTPMethod `json:"httpMethod"`
			Headers        KeyValPair         `json:"headers"`
			RawQueryParams json.RawMessage    `json:"queryParams"`
			RawUrlParams   json.RawMessage    `json:"urlParams"`
			RawPayload     json.RawMessage    `json:"payload"`
		}
		if err := json.Unmarshal(req, &rawProps); err != nil {
			return err
		}
		reqCmd := NewReqCmd("", rMgr).
			SetUrl(rawProps.Url).
			SetMethod(rawProps.HttpMethod).
			SetHeaders(rawProps.Headers)

		reqCmd.initializeVlds(
			rawProps.RawUrlParams,
			rawProps.RawQueryParams,
			rawProps.RawPayload,
		)
		reqCmd.register(rawProps.Cmd, hdlr, rMgr)
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

func (r *ReqCmd) register(
	cmdWithSubCmds string,
	hdlr *cmd.CmdHandler,
	rMgr *network.RequestManager,
) {
	if strings.Trim(cmdWithSubCmds, " ") == "" {
		panic("cannot register request cmd without a command")
	}

	var command cmd.AsyncCmd
	segments := strings.Fields(cmdWithSubCmds)
	rootCmd := segments[0]
	cmdRegistry := hdlr.GetCmdRegistry()
	r.Mgr = rMgr

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
		command = NewReqCmd(rootCmd, rMgr)
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
			subCmd = subCmd.AddSubCmd(NewReqCmd(string(token), rMgr))
		}
	}
}

func (rc *ReqCmd) initializeVlds(
	rawUrlParams, rawQueryParams, rawPayload json.RawMessage,
) {
	rc.UrlParams.initialize(rawUrlParams)
	rc.ReqPropsSchema.QueryParams.initialize(rawQueryParams)
	rc.ReqPropsSchema.Payload.initialize(rawPayload)
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
		if err := validate(key, value, rc.ReqPropsSchema.Payload, cmdParams.Payload); err != nil {
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
	if len(cmdParams.Payload) > 0 {
		payloadBytes, err := json.Marshal(cmdParams.Payload)
		if err != nil {
			return nil, fmt.Errorf("error marshalling payload: %w", err)
		}
		reqBody = bytes.NewReader(payloadBytes)
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
	if rc.RequestDraft == nil {
		return
	}
	schema := rc.ReqPropsSchema
	params := []ValidationSchema{schema.QueryParams, schema.UrlParams, schema.Payload}
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
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(respBytes, &target)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return nil
}

func (rc *ReqCmd) handleSuccessfulResponse(taskStatus *cmd.TaskStatus, result network.Update) {
	defer result.Resp().Body.Close()

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

func (r *ReqCmd) PopulateSchemasFromDraft() {
	if r.RequestDraft == nil {
		return
	}

	if r.ReqPropsSchema == nil {
		r.ReqPropsSchema = &ReqPropsSchema{
			QueryParams: make(ValidationSchema),
			Payload:     make(ValidationSchema),
			UrlParams:   make(ValidationSchema),
		}
	}

	r.populateQuerySchemaFromDraft()
	r.ReqPropsSchema.Payload = populateSchemaFromJSONString(r.GetPayload())
}

func (req *ReqCmd) cleanup() {
}

func (r *ReqCmd) populateQuerySchemaFromDraft() {
	schema := r.ReqPropsSchema
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

	r.IterateQueryParams(handlerFunc)
	schema.Payload = populateSchemaFromJSONString(r.RequestDraft.GetPayload())
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
		objSchema := make(ObjValidation)
		for key, subValue := range v {
			objSchema[key] = inferTypeSchema(subValue)
		}
		return objSchema

	case []any:
		var arrSchema ArrValidation

		if len(v) > 0 {
			arrSchema = make(ArrValidation, len(v))
			for idx, item := range v {
				if item != nil {
					arrSchema[idx] = inferTypeSchema(item)
					break
				}
			}
		}
		return arrSchema

	default:
		return &StrValidations{}
	}
}

func populateSchemaFromJSONString(jsonString string) ValidationSchema {
	schema := make(ValidationSchema)
	var payloadMap map[string]any

	if jsonString == "" {
		return schema
	}

	if err := json.Unmarshal([]byte(jsonString), &payloadMap); err != nil {
		fmt.Printf("Error unmarshalling JSON: %v\n", err)
		return schema
	}

	for key, value := range payloadMap {
		schema[key] = inferTypeSchema(value)
	}

	return schema
}

func GetRawReqCfg() ([]byte, error) {
	cfgFilePath := config.GetAppCfg().CfgFilePath()
	return os.ReadFile(cfgFilePath)
}

func marshalReqCmdCfg(reqCmd *ReqCmd, cmd string) ([]byte, error) {
	var reqCmdCfg struct {
		HttpMethod   string                `json:"httpMethod"`
		Cmd          string                `json:"cmd"`
		Url          string                `json:"url"`
		QueryParams  map[string]Validation `json:"queryParams"`
		UrlParams    map[string]Validation `json:"urlParams"`
		Payload      map[string]Validation `json:"payload"`
		RequestDraft *network.RequestDraft `json:"requestDraft"`
	}

	reqCmdCfg.Cmd = cmd
	reqCmdCfg.Url = reqCmd.Url
	reqCmdCfg.HttpMethod = string(reqCmd.Method)
	reqCmdCfg.QueryParams = reqCmd.ReqPropsSchema.QueryParams
	reqCmdCfg.UrlParams = reqCmd.ReqPropsSchema.UrlParams
	reqCmdCfg.Payload = reqCmd.ReqPropsSchema.Payload
	reqCmdCfg.RequestDraft = reqCmd.RequestDraft

	encodedJson, err := json.MarshalIndent(reqCmdCfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return encodedJson, nil
}

func SaveNewReqCmd(reqCmd *ReqCmd, cmd string) error {
	encJsonCfg, err := marshalReqCmdCfg(reqCmd, cmd)
	if err != nil {
		return fmt.Errorf("failed to encode request config %w", err)
	}
	existingCfg, err := GetRawReqCfg()
	if err != nil {
		return fmt.Errorf("failed to get existing config %w", err)
	}
	var iCfg struct {
		Requests []json.RawMessage `json:"requests"`
	}

	err = json.Unmarshal(existingCfg, &iCfg)
	if err != nil {
		return fmt.Errorf("invalid 'config.json' file: %w", err)
	}

	var fullCfg map[string]any
	err = json.Unmarshal(existingCfg, &fullCfg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal existin cfg: %w", err)
	}

	iCfg.Requests = append(iCfg.Requests, encJsonCfg)
	fullCfg["requests"] = iCfg.Requests

	finalCfg, err := json.MarshalIndent(fullCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling req config json %w", err)
	}

	cfgFilePath := config.GetAppCfg().CfgFilePath()
	if err := os.WriteFile(cfgFilePath, finalCfg, 0644); err != nil {
		return fmt.Errorf("failed to update 'config.json' %w", err)
	}

	return nil
}

// func populateSchemaFromJSONString(jsonString string) ValidationSchema {
// 	schema := make(ValidationSchema)
// 	var payloadMap map[string]any
//
// 	if jsonString == "" || json.Unmarshal([]byte(jsonString), &payloadMap) != nil {
// 		return schema
// 	}
//
// 	for key, value := range payloadMap {
// 		switch value.(type) {
// 		case string:
// 			schema[key] = &StrValidations{}
// 		case int, float64:
// 			if _, isInt := value.(int); isInt || float64(int(value.(float64))) == value.(float64) {
// 				schema[key] = &IntValidations{}
// 			} else {
// 				schema[key] = &FloatValidations{}
// 			}
// 		default:
// 			schema[key] = &StrValidations{}
// 		}
// 	}
// 	return schema
// }
