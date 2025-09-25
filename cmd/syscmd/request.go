package syscmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	*cmd.BaseCmd
	*ReqProps
}

type ReqData struct {
	queryParams KeyValPair
	Headers     KeyValPair
	Payload     map[string]interface{}
}

type RequestManager struct {
	Tracker       *RequestTracker
	Client        *http.Client
	CommonHeaders http.Header
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

// MakeRequest sends a request and returns its ID and a completion channel.
func (rm *RequestManager) MakeRequest(req *http.Request) (string, Done, error) {
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

	go func(rm *RequestManager, id string, r *http.Request) {
		// Use a defer statement to ensure close(done) is always called
		defer close(done)

		start := time.Now()
		resp, err := rm.Client.Do(r)
		if err != nil {
			trackerReq.Status = StatusError // Update status on error
			return
		}
		defer resp.Body.Close()

		requestTime := time.Since(start)

		// Create the update and send it to the tracker's channel
		update := Update{
			reqId: id,
			resp:  resp,
		}
		rm.Tracker.updates <- update

		// Set the request time after the update is sent (optional, can be done in the update itself)
		trackerReq.RequestTime = requestTime

		log.Info("Worker completed request with ID: %s", id)

	}(rm, reqID, req)

	return reqID, done, nil
}

func ParseRawReqCfg(rawCfg config.RawCfg) error {
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
			&cmd.BaseCmd{
				Name_: "", //IMP: Since there could be multiple segments.
			},
			&ReqProps{
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
		reqCmd.register(rawProps.Cmd)
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

func (r *ReqCmd) register(cmdWithSubCmds string) {
	var command cmd.AsyncCmd
	segments := strings.Fields(cmdWithSubCmds)
	rootCmd := segments[0]

	if existingCmd, exists := cmd.GetCmdByName(rootCmd); exists {
		if existingAsyncCmd, ok := existingCmd.(cmd.AsyncCmd); ok {
			command = existingAsyncCmd
		} else {
			panic(fmt.Sprintf("another non async command already registered with name '%s'", rootCmd))
		}
	} else {
		command = &ReqCmd{BaseCmd: &cmd.BaseCmd{
			Name_: rootCmd,
		}}
		cmd.RegisterCmd(command)
	}

	remainingTkns, subCmd := command.WalkTillLastSubCmd(util.StrArrToRune(segments[1:]))
	for i, token := range remainingTkns {
		isLast := i == len(segments)-1
		if isLast {
			subCmd.AddSubCmd(r)
		} else {
			subCmd = subCmd.AddSubCmd(&ReqCmd{
				BaseCmd: &cmd.BaseCmd{
					Name_: string(token),
				},
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

func (rc *ReqCmd) ExecuteAsync(
	ctx context.Context,
	tokens []string,
) {
  // u := rc.GetCmdHandler().GetUpdateChan()
  // t := cmd.TaskStatus{
  //   Message: "Working on it..",
  // }
  // u <- t
  // go func() {
  //   defer close(u)
  //
  //   for i := range 5 {
  //     time.Sleep(2*time.Second)
  //     t.Message = fmt.Sprintf("step '%d' done..", i+1)
  //     u <- t
  //     if i == 3 {
  //       t.Message = "I failed mannnn"
  //       t.Error = errors.New("Shit")
  //       t.Done = true
  //       u <- t
  //     }
  //   }
  // }()
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

func outputResp(resp map[string]interface{}) {
	var jsonBuffer bytes.Buffer
	enc := json.NewEncoder(&jsonBuffer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(resp)

	lexer := lexers.Get("json")
	respStr := jsonBuffer.String()
	highlightedJSON := highlightText(respStr, lexer)

	fmt.Print(highlightedJSON)
	// cmd.Shell.Refresh()
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
