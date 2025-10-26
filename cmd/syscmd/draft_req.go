package syscmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/network"
)

const (
	CmdDraftReqName = "$draft_req"
)

type draftReqCmd struct {
  *BaseReqCmd
}

func (d *draftReqCmd) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	rDraft := network.NewRequestDraft()
	if d.Mgr == nil {
		return cmdCtx.Ctx, errors.New("failed to draft request, manager unavailable")
	}

  mgr := d.Mgr
	mgr.AddDraftRequest(cmdCtx.ID(), rDraft)
  draftOffset := len(mgr.GetRequestDrafts(cmdCtx.ID()))

  prompt := fmt.Sprintf("Request Draft (%d)", draftOffset)
	d.GetCmdHandler().SetPrompt(prompt, "")
	return cmdCtx.Ctx, nil
}
