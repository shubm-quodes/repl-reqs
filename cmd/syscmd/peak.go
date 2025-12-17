package syscmd

import (
	"context"
	"fmt"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/config"
)

const (
	// Root cmd
	CmdPeakName = "$peak"

	// Sub cmds
	CmdPeakVarName = "var"
)

type CmdPeak struct {
	*cmd.BaseNonModeCmd
}

type CmdPeakVar struct {
	*cmd.BaseNonModeCmd
}

func (cp *CmdPeakVar) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens
	if len(tokens) == 0 {
		return ctx, fmt.Errorf("please specify var name")
	}
	varName := tokens[0]
	envMgr := config.GetEnvManager()
	hdlr := cp.GetCmdHandler()

	if value, exists := envMgr.GetVar(varName); exists {
		hdlr.OutF(cmdCtx, "📦 %s: %s\n", varName, value)
		return ctx, nil
	} else {
		return ctx, fmt.Errorf("'%s': no such var", varName)
	}
}

func (cp *CmdPeakVar) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	var search string
	if len(tokens) > 0 {
		search = string(tokens[0])
	}

	envMgr := config.GetEnvManager()
	return envMgr.GetMatchingVars(search), len(search)
}
