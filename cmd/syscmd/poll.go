package syscmd

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/network"
)

const (
	CmdPollName = "$poll"
)

type CmdPoll struct {
	*BaseReqCmd
}

/*
- Single argument is treated as the poll condition
- In case of multiple args, the last arg would be treated as the condition, while all others preceding it are considered as commands
*/
func (cp *CmdPoll) ExecuteAsync(cmdCtx *cmd.CmdCtx) {
	rd, condition, err := cp.determinePollReqDraft(
		cmdCtx.ExpandedTokens,
		cmdCtx,
	)

	t := cmdCtx.Task
	if err != nil {
		t.Fail(err)
	}

	req, err := rd.Finalize()
	if err != nil {
		t.Fail(err)
	}

	response, err := cp.Poll(req, condition)
	if err != nil {
		t.Fail(err)
	} else {
		t.AppendOutput(getFormattedResp(response) + "\n" + response.Status)
		t.CompleteWithMessage("Polling complete", response)
	}
}

func (cp *CmdPoll) Poll(req *http.Request, condition string) (*http.Response, error) {
	c, err := network.NewCondition(condition)
	if err != nil {
		return nil, err
	}

	p := network.NewPoller(req, c)
	return p.Poll()
}

func (cp *CmdPoll) determinePollReqDraft(
	tokens []string,
	cmdCtx *cmd.CmdCtx,
) (*network.RequestDraft, string, error) {
	if len(tokens) == 0 {
		return nil, "", errors.New(
			"neither condition nor command with a condition was specified ",
		)
	}

	if len(tokens) == 1 {
		draft := cp.Mgr.PeakRequestDraft(cmdCtx.ID())
		if draft == nil {
			return nil, "", errors.New("no request drafts to poll")
		} else {
			return draft, tokens[0], nil
		}
	}

	// Multiple args were specified.
	cmd := tokens[:len(tokens)-1]
	hdlr := cp.GetCmdHandler()
	c, unusedTokens := hdlr.ResolveCommandFromRoot(cmd)

	if len(unusedTokens) == 0 {
		return nil, "", errors.New("please specify a 'poll condition' :/")
	}

	if len(unusedTokens) > 1 { // Hmm... possibly wrong cmd
		return nil, "", fmt.Errorf(
			"invalid command '%s' :/",
			strings.Join(cmd, " "),
		)
	}

	// We're here, this means exactly one unused token is left, we'll assume it to be the 'condition'
	if rCmd, ok := c.(*ReqCmd); !ok {
		return nil, "", fmt.Errorf(
			"'%s' is not a request command",
			strings.Join(cmd, " "),
		)
	} else {
		return rCmd.RequestDraft, unusedTokens[0], nil
	}
}
