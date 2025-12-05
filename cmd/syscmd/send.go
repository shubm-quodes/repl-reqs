package syscmd

import (
	"fmt"

	"github.com/shubm-quodes/repl-reqs/cmd"
)

const (
	CmdSendName = "$send"
)

type CmdSend struct {
	*ReqCmd
}

func (s *CmdSend) ExecuteAsync(cmdCtx *cmd.CmdCtx) {
	t := cmdCtx.Task
	draft := s.Mgr.PeakRequestDraft(cmdCtx.ID())

	if draft == nil {
		t.Fail(
			fmt.Errorf("no drafts, start drafting requests using %s command", CmdDraftReqName),
		)
		return
	}

	if req, err := draft.Finalize(); err != nil {
		t.Fail(err)
	} else {
		s.MakeRequest(req, t)
	}
}

func (s *CmdSend) AllowInModeWithoutArgs() bool {
	return false
}
