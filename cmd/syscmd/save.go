package syscmd

import (
	"context"
	"errors"

	"github.com/shubm-quodes/repl-reqs/cmd"
)

const (
	CmdSaveName = "$save"
)

type CmdSave struct {
	*BaseReqCmd
}

// Upserts a new request command
func (s *CmdSave) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	tokens, ctx := cmdCtx.ExpandedTokens, cmdCtx.Ctx
	if len(tokens) == 0 {
		return ctx, errors.New(
			`please specify a command name, multiple words can be separated by spaces and will be treated as sub commands`,
		)
	}

	reqCmd := NewReqCmd(tokens[len(tokens)-1], s.Mgr)
	draft := s.Mgr.PeakRequestDraft(cmdCtx.ID())
	if draft == nil {
		return ctx, errors.New("failed to find a request draft to save/update")
	}

	reqCmd.RequestDraft = draft
	hdlr := s.GetCmdHandler()
	hdlr.Inject(reqCmd)

	if err := UpsertReqCfg(reqCmd, s.Mgr, tokens); err != nil {
		return ctx, err
	}

	hdlr.Out(cmdCtx, "request saved successfully ✅")
	return ctx, nil
}

func (s *CmdSave) AllowInModeWithoutArgs() bool {
	return false
}

func (s *CmdSave) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	return s.SuggestWithoutParams(tokens)
}
