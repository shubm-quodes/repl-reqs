package syscmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/shubm-quodes/repl-reqs/cmd"
	c "github.com/shubm-quodes/repl-reqs/config"
	"github.com/shubm-quodes/repl-reqs/network"
	"github.com/shubm-quodes/repl-reqs/util"
)

const (
	NotAvlConst = "<N/A>"

	// Top-level command names
	CmdSetName = "$set"

	// Subcommand names for '$set'
	CmdEnvName          = "env"
	CmdVarName          = "var"
	CmdURLName          = "url"
	CmdQueryName        = "query"
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
	*BaseReqCmd
}

type CmdHeader struct {
	*BaseReqCmd
}

type CmdMultiHeaders struct {
	*BaseReqCmd
	req *http.Request
	mu  *sync.Mutex
}

type CmdHTTPVerb struct {
	*BaseReqCmd
}

type CmdPayload struct {
	*BaseReqCmd
}

type CmdQuery struct {
	*BaseReqCmd
}

type CmdPrompt struct {
	*cmd.BaseCmd
}

type CmdMascot struct {
	*cmd.BaseCmd
}

type CmdSet struct {
	*cmd.BaseCmd
}

var CurrReqIdx int

func (ch *CmdHeader) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ctx, tokens := cmdCtx.Ctx, cmdCtx.RawTokens
	if len(tokens) < 2 {
		return ctx, errors.New("please provide header [key] [val]")
	}

	key, val := tokens[0], tokens[1]
	reqDraft := ch.Mgr.PeakRequestDraft(cmdCtx.ID())

	reqDraft.SetHeader(key, val)
	return ctx, nil
}

func (u *CmdURL) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	if u.Mgr == nil {
		return cmdCtx.Ctx, errors.New("failed to set url, manager unavailable")
	}
	if len(cmdCtx.ExpandedTokens) == 0 {
		return cmdCtx.Ctx, errors.New("please specify url :/")
	}

	url := cmdCtx.ExpandedTokens[0]
	draft := u.Mgr.PeakRequestDraft(cmdCtx.ID())

	if draft != nil {
		draft.SetUrl(url)
		setReqDraftPrompt(u.GetCmdHandler(), draft)
		return cmdCtx.Ctx, nil
	}

	return cmdCtx.Ctx, errors.New("no request to draft")
}

func (cmh *CmdMultiHeaders) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ctx := cmdCtx.Ctx
	handler := cmh.GetCmdHandler()

	if handler.GetCurrentModeCmd() != cmh {
		cmh.activateMultiHeaderMode()
		return ctx, nil
	} else {
		//TODO: Implement context logic here to pickup request data.
		return ctx, nil
	}
}

func (cmh *CmdQuery) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	tokens, ctx := cmdCtx.RawTokens, cmdCtx.Ctx
	if len(tokens) < 2 {
		return ctx, errors.New("query 'key' and 'value' are required")
	}

	key, val := tokens[0], strings.Join(tokens[1:], " ")
	rMgr := cmh.Mgr
	draft := rMgr.PeakRequestDraft(cmdCtx.ID())
	draft.SetQueryParam(key, val)
	return ctx, nil
}

func (cmh *CmdMultiHeaders) setHeader(key, val string) error {
	if util.AreEmptyStrs(key, val) {
		return errors.New("please provide both header key and value")
	}
	cmh.req.Header.Set(key, val)
	return nil
}

func (cmh *CmdMultiHeaders) activateMultiHeaderMode() {
	handler := cmh.GetCmdHandler() // here is says cmh.GetCmdHandler undefined
	handler.PushCmdMode("$multi-headers", cmh, false)
}

func (chv *CmdHTTPVerb) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	tokens, ctx := cmdCtx.ExpandedTokens, cmdCtx.Ctx
	if len(tokens) == 0 {
		return ctx, errors.New("please specify httpverb")
	}

	httpVerb := tokens[0]
	if !network.IsValidHttpVerb(network.HTTPMethod(httpVerb)) {
		return ctx, fmt.Errorf(`invalid httpVerb "%s"`, httpVerb)
	}

	draft := chv.Mgr.PeakRequestDraft(cmdCtx.ID())
	if draft == nil {
		return ctx, errors.New("request draft not found")
	}

	draft.SetMethod(network.HTTPMethod(httpVerb))
	setReqDraftPrompt(chv.GetCmdHandler(), draft)
	return ctx, nil
}

func (ec *CmdEnv) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	tokens, ctx := cmdCtx.ExpandedTokens, cmdCtx.Ctx
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

func (vc *CmdVar) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	tokens, ctx := cmdCtx.ExpandedTokens, cmdCtx.Ctx
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

func (pc *CmdPrompt) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	tokens, ctx := cmdCtx.ExpandedTokens, cmdCtx.Ctx
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

func (cm *CmdMascot) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	tokens, ctx := cmdCtx.ExpandedTokens, cmdCtx.Ctx
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

func setReqDraftPrompt(hdlr cmd.CmdHandler, draft *network.RequestDraft) {
	var (
		prompt string
		url    = draft.GetUrl()
		cfg    = hdlr.GetAppCfg()
	)

	if cfg.TruncatePrompt() && len(url) > int(cfg.MaxPromptChars()) {
		prompt = color.HiYellowString("..." + url[len(url)-20:])
	} else {
		prompt = color.HiYellowString(url)
	}

	if draft.GetMethod() != "" {
		prompt = fmt.Sprintf(
			"%s [%s]",
			prompt,
			color.GreenString(string(draft.GetMethod())),
		)
	}

	hdlr.SetPrompt(prompt, "")
}
