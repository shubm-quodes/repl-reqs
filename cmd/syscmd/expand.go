package syscmd

import (
	"context"
	"fmt"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/config"
	"github.com/shubm-quodes/repl-reqs/util"
)

const (
	// Root cmd
	CmdExpandName = "$expand"

	// Sub cmds
	CmdExpandVarName = "var"
)

type CmdExpand struct {
	*cmd.BaseNonModeCmd
}

type CmdExpandVar struct {
	*cmd.BaseNonModeCmd
}

func (cp *CmdExpandVar) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens
	if len(tokens) == 0 {
		return ctx, fmt.Errorf("please specify var name")
	}
	varName := tokens[0]
	envMgr := config.GetEnvManager()
	hdlr := cp.GetCmdHandler()

	if value, exists := envMgr.GetVar(varName); exists {
		value, err := util.ReplaceStrPattern(
			value,
			config.VarPattern,
			config.GetEnvManager().GetActiveEnvVars(),
		)

		if err != nil {
			return ctx, fmt.Errorf("failed to expand var: %s", err.Error())
		}

		hdlr.OutF(cmdCtx, "📦 %s: %s\n", varName, value)
		return ctx, nil
	} else {
		return ctx, fmt.Errorf("'%s': no such var", varName)
	}
}

func (cp *CmdExpandVar) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	var search string
	if len(tokens) > 0 {
		search = string(tokens[0])
	}

	envMgr := config.GetEnvManager()
	return envMgr.GetMatchingVars(search), len(search)
}
