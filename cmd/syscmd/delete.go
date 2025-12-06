package syscmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/config"
	"github.com/shubm-quodes/repl-reqs/util"
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
		return ctx, errors.New("please specify var name(s)❗️")
	}

	envMgr := config.GetEnvManager()
	for _, variable := range tokens {
		if _, exists := envMgr.GetVar(variable); !exists {
			return ctx, fmt.Errorf("'%s' doesn't seem to exist 😬", variable)
		} else {
			envMgr.DeleteVar(variable)
		}
	}

	dltVar.GetCmdHandler().
		OutF(cmdCtx, "done, '%s' gone forever lost in the clouds ☁️\n", strings.Join(tokens, ", "))
	return ctx, nil
}

func (dltSeq *CmdDeleteSeq) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens
	if len(tokens) == 0 {
		return ctx, errors.New("please specify sequence name(s)❗️")
	}

	for _, seq := range tokens {
		err := dltSeq.GetCmdHandler().DiscardSequence(seq)
		if err != nil {
			return ctx, err
		}
	}

	return ctx, nil

}

func (dltSeq *CmdDeleteSeq) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	var (
		alreadySuggested [][]rune
		search           string
	)

	hdlr := dltSeq.GetCmdHandler()
	if len(tokens) >= 1 {
		lastToken := string(tokens[len(tokens)-1])
		if _, err := hdlr.GetSequence(lastToken); err != nil {
			search = lastToken
		} else {
			alreadySuggested = tokens[0:]
		}
	}

	suggestions := util.MapSlice(
		hdlr.SuggestSequences(search),
		func(elem []rune, idx int) []rune { return util.TrimRunes(elem) },
	)

	return util.RuneSliceDiff(suggestions, alreadySuggested), len(search)
}

func (dltVar *CmdDeleteVar) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	var (
		alreadySuggested [][]rune
		search           string
	)

	envMgr := config.GetEnvManager()
	if len(tokens) >= 1 {
		lastToken := string(tokens[len(tokens)-1])
		if _, found := envMgr.GetVar(lastToken); !found {
			search = lastToken
		} else {
			alreadySuggested = tokens[0:]
		}
	}

	suggestions := util.MapSlice(
		envMgr.GetMatchingVars(search),
		func(elem []rune, idx int) []rune { return util.TrimRunes(elem) },
	)

	return util.RuneSliceDiff(suggestions, alreadySuggested), len(search)
}
