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
	t := s.GetTaskStatus()
	draft := s.Mgr.PeakRequestDraft(cmdCtx.ID())
	updateChan := s.GetCmdHandler().GetUpdateChan()

	if draft == nil {
		t.SetError(
			fmt.Errorf("no drafts, start drafting requests using %s command", CmdDraftReqName),
		)
    t.SetDone(true)
		updateChan <- (*t)
    return
	}

	if req, err := draft.Finalize(); err != nil {
		t.SetError(err)
		updateChan <- (*t)
	} else {
		s.MakeRequest(req)
	}
}

func (s *CmdSend) AllowInModeWithoutArgs() bool {
	return false
}
