package syscmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/nodding-noddy/repl-reqs/cmd"
	c "github.com/nodding-noddy/repl-reqs/config"
	"github.com/nodding-noddy/repl-reqs/util"
)

const (
	NotAvlConst = "<N/A>"

	// Top-level command names
	CmdSetName = "$set"

	// Subcommand names for '$set'
	CmdEnvName          = "env"
	CmdVarName          = "var"
	CmdURLName          = "url"
	CmdHeaderName       = "header"
	CmdMultiHeadersName = "multi-headers"
	CmdHTTPVerbName     = "httpVerb"
	CmdPayloadName      = "payload"
	CmdPromptName       = "prompt"
	CmdMascotName       = "mascot"
)

type CmdEnv struct {
	*cmd.BaseCmd
}

type CmdVar struct {
	*cmd.BaseCmd
}

type CmdURL struct {
	*cmd.BaseCmd
}

type CmdHeader struct {
	*cmd.BaseCmd
}

type CmdMultiHeaders struct {
	*cmd.BaseCmd
	req *http.Request
	mu  *sync.Mutex
}

type CmdHTTPVerb struct {
	*cmd.BaseCmd
}

type CmdPayload struct {
	*cmd.BaseCmd
}

type CmdPrompt struct {
	*cmd.BaseCmd
}

type CmdMascot struct {
	*cmd.BaseCmd
}

type setCmd struct {
	*cmd.BaseCmd
}

var CurrReqIdx int

type Vars map[string]interface{}

var variables = make(Vars)

func init() {
	s := &setCmd{&cmd.BaseCmd{Name_: CmdSetName}}
	s.AddSubCmd(&CmdEnv{&cmd.BaseCmd{Name_: CmdEnvName}})
	s.AddSubCmd(&CmdVar{&cmd.BaseCmd{Name_: CmdVarName}})
	s.AddSubCmd(&CmdURL{&cmd.BaseCmd{Name_: CmdURLName}})
	s.AddSubCmd(&CmdHeader{&cmd.BaseCmd{Name_: CmdHeaderName}})
	s.AddSubCmd(&CmdMultiHeaders{BaseCmd: &cmd.BaseCmd{Name_: CmdMultiHeadersName}})
	s.AddSubCmd(&CmdHTTPVerb{&cmd.BaseCmd{Name_: CmdHTTPVerbName}})
	s.AddSubCmd(&CmdPayload{&cmd.BaseCmd{Name_: CmdPayloadName}})
	s.AddSubCmd(&CmdPrompt{&cmd.BaseCmd{Name_: CmdPromptName}})
	s.AddSubCmd(&CmdMascot{&cmd.BaseCmd{Name_: CmdMascotName}})

	cmd.RegisterSysCmd(s)
}

func (cmh *CmdMultiHeaders) Execute(
	ctx context.Context,
	tokens []string,
) (context.Context, error) {
	handler := cmh.GetCmdHandler()

	if handler.GetCurrentCmdMode() != cmh {
		cmh.activateMultiHeaderMode()
		return ctx, nil
	} else {
		//TODO: Implement context logic here to pickup request data.
		return ctx, nil
	}
}

func (cmh *CmdMultiHeaders) setHeader(key, val string) error {
	if util.AreEmptyStrs(key, val) {
		errors.New("please provide both header key and value")
	}
	cmh.req.Header.Set(key, val)
	return nil
}

func (cmh *CmdMultiHeaders) activateMultiHeaderMode() {
	handler := cmh.GetCmdHandler() // here is says cmh.GetCmdHandler undefined
	handler.SetCurrentCmdMode("$multi-headers", cmh)
}

func (chv *CmdHTTPVerb) Execute(
	ctx context.Context,
	tokens []string,
) (context.Context, error) {
	if len(tokens) == 0 {
		return ctx, errors.New("please specify httpverb")
	}
	httpVerb := tokens[0]
	if !isValidHttpVerb(HTTPMethod(httpVerb)) {
		return ctx, fmt.Errorf(`invalid httpVerb "%s"`, httpVerb)
	}
	return ctx, nil
}

func (ec *CmdEnv) Execute(ctx context.Context, tokens []string) (context.Context, error) {
	if len(tokens) == 0 {
		return ctx, errors.New("please specify environment name")
	}
	env := tokens[0]

	mgr := c.GetEnvManager()
	mgr.SetActiveEnv(env)

  ec.GetCmdHandler().UpdatePromptEnv()
	fmt.Printf(`Environment now set to "%s"`+"\n", env)
	return ctx, nil
}

func (vc *CmdVar) Execute(ctx context.Context, tokens []string) (context.Context, error) {
	if len(tokens) < 2 {
		return ctx, errors.New(
			"failed to set variable: name & value are required",
		)
	}

	mgr := c.GetEnvManager()
	name, val := tokens[0], strings.Join(tokens[1:], " ")

	mgr.SetVar(name, val)
	fmt.Printf("'%s' now set to '%s'\n", name, val)

	return ctx, nil
}

func getVar[T interface{}](name string) (T, bool) {
	found, ok := variables[name].(T)
	return found, ok
}

func (pc *CmdPrompt) Execute(
	ctx context.Context,
	tokens []string,
) (context.Context, error) {
	if len(tokens) == 0 {
		return ctx, errors.New("please specify prompt")
	}

	prompt := tokens[0]
	cfg := pc.GetCmdHandler().GetAppCfg()
	if err := cfg.UpdateDefaultPrompt(prompt); err != nil {
		return ctx, err
	}

	h := pc.GetCmdHandler()
	h.SetPrompt(prompt, "")
	return ctx, nil
}

func (cm *CmdMascot) Execute(
	ctx context.Context,
	tokens []string,
) (context.Context, error) {
	if len(tokens) == 0 {
		return ctx, errors.New("please specify mascot")
	}
	mascot := tokens[0]
	cfg := cm.GetCmdHandler().GetAppCfg()
	if err := cfg.UpdateDefaultMascot(mascot); err != nil {
		return ctx, err
	}

	prompt := cfg.GetPrompt()
	h := cm.GetCmdHandler()
	h.SetPrompt(prompt, mascot)

	return ctx, nil
}
