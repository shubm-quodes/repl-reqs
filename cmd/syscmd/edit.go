package syscmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/network"
	"github.com/shubm-quodes/repl-reqs/util"
)

const (
	// Root Cmd
	CmdEditName = "$edit"

	// Sub commands
	CmdEditReqName = "req"
	CmdEditSeqName = "sequence"
)

type CmdEdit struct {
	*BaseReqCmd
}

type CmdEditReq struct {
	*BaseReqCmd
}

func (er *CmdEditReq) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens

	if len(tokens) == 0 {
		return ctx, errors.New("please specify req name")
	}

	hdlr := er.GetCmdHandler()
	c, exists := hdlr.GetCmdRegistry().GetCmdByName(tokens[0])
	if !exists {
		return ctx, fmt.Errorf("invalid request command '%s'", strings.Join(tokens, " "))
	}

	remainingTokens, c := cmd.Walk(c, c.GetSubCmds(), util.StrArrToRune(tokens[1:]))
	if len(remainingTokens) > 0 {
		return ctx, fmt.Errorf("incomplete/invalid request command '%s'", strings.Join(tokens, " "))
	}

	req, ok := c.(*ReqCmd)
	if !ok {
		return ctx, fmt.Errorf("not a request command")
	}

	rd := req.RequestDraft
	if rd == nil {
		rd = network.NewRequestDraft()
		req.RequestDraft = rd
	}

	return ctx, rd.EditAsToml()
}

func (er *CmdEditReq) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	if len(tokens) == 0 {
		return nil, 0
	}

	firstToken := tokens[0]
	if !er.isValidEdtReqCmdToken(firstToken) {
		return nil, 0
	}

	hdlr := er.GetCmdHandler()
	cmd, found := hdlr.GetCmdRegistry().GetCmdByName(string(firstToken))
	if !found {
		return hdlr.SuggestCmds(tokens)
	}

	reqCmd, ok := cmd.(*ReqCmd)
	if !found || !ok {
		return nil, 0
	}

	finalCmd, remainingTokens := reqCmd.walkCommandTree(tokens)

	if finalCmd == nil {
		return nil, 0
	}

	if finalCmd == reqCmd {
		remainingTokens = remainingTokens[1:]
	}

	return finalCmd.BaseCmd.GetSuggestions(remainingTokens)
}

func (er *CmdEditReq) isValidEdtReqCmdToken(token []rune) bool {
	return len(token) != 0 || token[0] != '$'
}
