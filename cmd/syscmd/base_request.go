package syscmd

import (
	"strings"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/network"
)

type BaseReqCmd struct {
	*cmd.BaseCmd
	Mgr *network.RequestManager
}

func NewBaseReqCmd(name string) *BaseReqCmd {
	return &BaseReqCmd{
		BaseCmd: &cmd.BaseCmd{
			Name_: name,
		},
	}
}

func (brc *BaseReqCmd) SetReqMgr(mgr *network.RequestManager) {
	brc.Mgr = mgr
}

func (rc *BaseReqCmd) SuggestWithoutParams(tokens [][]rune) ([][]rune, int) {
	if len(tokens) == 0 {
		tokens = make([][]rune, 1)
	}

	firstToken := tokens[0]
	if !rc.isValidEdtReqCmdToken(firstToken) {
		return nil, 0
	}

	hdlr := rc.GetCmdHandler()
	cmd, found := hdlr.GetCmdRegistry().GetCmdByName(string(firstToken))
	if !found {
		return rc.suggestRootCmds(tokens)
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

func (rc *BaseReqCmd) suggestRootCmds(tokens [][]rune) ([][]rune, int) {
	suggestions, offset := rc.GetCmdHandler().SuggestCmds(tokens)
	var filteredSugg [][]rune

	for _, s := range suggestions {
		if s[0] != '$' { // Filter sys cmds
			filteredSugg = append(filteredSugg, s)
		}
	}

	return filteredSugg, offset
}

func (rc *BaseReqCmd) isValidEdtReqCmdToken(token []rune) bool {
	return len(token) == 0 || token[0] != '$'
}

func (rc *BaseReqCmd) getSearchQuery(remainingTkns [][]rune) []rune {
	if len(remainingTkns) == 0 {
		return nil
	}

	lastToken := string(remainingTkns[len(remainingTkns)-1])

	if parts := strings.SplitN(lastToken, "=", 2); len(parts) != 2 {
		return []rune(lastToken)
	}

	return nil
}

func (rc *BaseReqCmd) AllowInModeWithoutArgs() bool {
	return false
}
