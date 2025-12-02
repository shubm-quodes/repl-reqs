package syscmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shubm-quodes/repl-reqs/cmd"
)

const (
	CmdSaveName = "$save"
)

type CmdSave struct {
	*BaseReqCmd
}

func (s *CmdSave) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	tokens, ctx := cmdCtx.ExpandedTokens, cmdCtx.Ctx
	if len(tokens) == 0 {
		return ctx, errors.New(
			`please specify a command name, multiple words can be separated by spaces and will be treated as sub commands`,
		)
	}

	reqCmd := NewReqCmd(tokens[0], s.Mgr)
	reqCmd.RequestDraft = s.Mgr.PeakRequestDraft(cmdCtx.ID())
	reqCmd.PopulateSchemasFromDraft()

	hdlr := s.GetCmdHandler()

	reqCmd.register(strings.Join(tokens, " "), hdlr, s.Mgr)
	if err := UpsertReqCfg(reqCmd); err != nil {
		return ctx, err
	}

	hdlr.Inject(reqCmd)
	fmt.Println("request saved successfully")
	return ctx, nil
}

func (s *CmdSave) AllowInModeWithoutArgs() bool {
	return false
}
