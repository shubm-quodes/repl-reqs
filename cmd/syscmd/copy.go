package syscmd

import (
	"context"
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/log"
	"github.com/shubm-quodes/repl-reqs/util"
)

const (
	CmdCopyName = "$copy"

	// Sub cmds
	CmdCopyResponseBodyName = "response_body"
)

type CmdCopy struct {
	*cmd.BaseCmd
}

type CmdCopyRespBody struct {
	*BaseReqCmd
}

func (cp *CmdCopyRespBody) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	tr, err := cp.Mgr.PeakTrackerRequest(cmdCtx.ID())
	if err != nil {
		return cmdCtx.Ctx, err
	}

	body, err := util.ReadAndResetIoCloser(&tr.ResponseBody)
	if err != nil {
		log.Debug("failed to read response body %s", err.Error())
		return cmdCtx.Ctx, fmt.Errorf("failed to read resp body")
	}

	formatted, err := util.ToIndentedPayload(body)
	if err != nil {
		return cmdCtx.Ctx, err
	}

	err = clipboard.WriteAll(string(formatted))
	if err == nil {
		fmt.Println("Roger, Copy that! 😉")
	}

	return cmdCtx.Ctx, err
}
