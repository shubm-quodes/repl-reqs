package syscmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/config"
)

const (
	// Root level cmd
	CmdDeleteName = "$delete"

	// Sub cmds
	CmdDeleteVarName = "var"
)

type CmdDelete struct {
	*cmd.BaseCmd
}

type CmdDeleteVar struct {
	*cmd.BaseCmd
}

func (dltVar *CmdDeleteVar) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens
	if len(tokens) == 0 {
		return ctx, errors.New("please specify var name❗️")
	}

	envMgr := config.GetEnvManager()
	varName := tokens[0]
	if _, exists := envMgr.GetVar(varName); !exists {
		return ctx, fmt.Errorf("'%s' doesn't seem to exist 😬", varName)
	}

	envMgr.DeleteVar(varName)
	fmt.Printf("done, '%s' gone forever lost in the clouds ☁️\n", varName)
	return ctx, nil
}

func (dltVar *CmdDelete) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	if len(tokens) >= 1 && dltVar.isSubCmd(string(tokens[0])) {
		var search string
		if len(tokens) > 1 {
			search = string(tokens[1])
		}
		return config.GetEnvManager().GetMatchingVars(search), len(search)
	} else {
		return dltVar.BaseCmd.GetSuggestions(tokens)
	}
}

func (dltVar *CmdDelete) isSubCmd(partial string) bool {
	_, ok := dltVar.SubCmds[partial]
	return ok
}
