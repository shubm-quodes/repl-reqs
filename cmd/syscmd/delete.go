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
	CmdDeleteSeqName = "sequence"
)

type CmdDelete struct {
	*cmd.BaseCmd
}

type CmdDeleteVar struct {
	*cmd.BaseCmd
}

type CmdDeleteSeq struct {
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

func (dltSeq *CmdDeleteSeq) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens
	if len(tokens) == 0 {
		return ctx, errors.New("please specify sequence name❗️")
	}

	return ctx, dltSeq.GetCmdHandler().DiscardSequence(tokens[0])
}

func (dltSeq *CmdDeleteSeq) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	var search string
	if len(tokens) == 1 {
		search = string(tokens[0])
	}

	return dltSeq.GetCmdHandler().SuggestSequences(search), len(search)
}

func (dltVar *CmdDeleteVar) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	var search string
	if len(tokens) == 1 {
		search = string(tokens[0])
	}

	return config.GetEnvManager().GetMatchingVars(search), len(search)
}
