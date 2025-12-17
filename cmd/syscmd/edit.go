package syscmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/config"
	"github.com/shubm-quodes/repl-reqs/log"
	"github.com/shubm-quodes/repl-reqs/network"
	"github.com/shubm-quodes/repl-reqs/util"
)

const (
	// Root Cmd
	CmdEditName = "$edit"

	// Sub commands
	CmdEditReqName      = "request"
	CmdEditJsonName     = "json"
	CmdEditXmlName      = "xml"
	CmdEditRespBodyName = "response_body"
	CmdEditSeqName      = "sequence"
)

type CmdEdit struct {
	*BaseReqCmd
}

type CmdEditReq struct {
	*BaseReqCmd
}

type CmdEditJSON struct {
	*BaseReqCmd
}

type CmdEditXml struct {
	*BaseReqCmd
}

type CmdEditRespBody struct {
	*BaseReqCmd
}

func (er *CmdEditReq) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens
	var rd *network.RequestDraft

	if len(tokens) == 0 {
		rd = er.Mgr.PeakRequestDraft(cmdCtx.ID())
		if rd == nil {
			return ctx, fmt.Errorf(
				"no drafts, start drafting requests using %s command",
				CmdDraftReqName,
			)
		}
	} else {
		c, remainingTokens := er.GetCmdHandler().ResolveCommandFromRoot(tokens)
		if c == nil || len(remainingTokens) > 0 {
			return ctx, fmt.Errorf("incomplete/invalid request cmd '%s'", strings.Join(tokens, " "))
		}

		if req, ok := c.(*ReqCmd); !ok {
			return ctx, fmt.Errorf("'%s' is not a request cmd", strings.Join(tokens, " "))
		} else {
			rd = req.RequestDraft
			if rd == nil {
				rd = network.NewRequestDraft()
				req.RequestDraft = rd
			}
		}
	}

	if er.Mgr.PeakRequestDraft(cmdCtx.ID()) != rd {
		er.Mgr.AddDraftRequest(cmdCtx.ID(), rd)
	}
	return ctx, rd.EditAsToml()
}

func (er *CmdEditRespBody) Execute(
	cmdCtx *cmd.CmdCtx,
) (context.Context, error) {
	trackerReq, err := er.Mgr.PeakTrackerRequest(cmdCtx.ID())

	if err != nil {
		return cmdCtx.Ctx, err
	}

	if trackerReq.ResponseBody == nil {
		return cmdCtx.Ctx, errors.New(
			"request still seems to be in progress, you could track it's status using $ls tasks cmd",
		)
	}

	return cmdCtx.Ctx, er.OpenEditor(trackerReq)
}

func (er *CmdEditRespBody) OpenEditor(req *network.TrackerRequest) error {
	contentType, ok := req.ResponseHeaders["Content-Type"]
	if !ok {
		return errors.New(
			"'content-type' header no found, failed to infer editor for response body",
		)
	}

	respBytes, err := util.ReadAndResetIoCloser(&req.ResponseBody)
	if err != nil {
		log.Debug(
			"failed to process response body for the last request: %w",
			err.Error(),
		)
		return errors.New(
			"failed to process response body for the last request",
		)
	}

	var edited []byte
	if strings.Contains(strings.ToLower(contentType[0]), "application/json") {
		edited, err = util.EditJsonRawWf(config.GetAppCfg().GetDefaultEditor(), respBytes)
	} else if strings.Contains(strings.ToLower(contentType[0]), "xml") {
		edited, err = util.EditXMLRawWf(config.GetAppCfg().GetDefaultEditor(), respBytes)
	} else {
		edited, err = util.EditTextRawWf(config.GetAppCfg().GetDefaultEditor(), respBytes)
	}

	if err != nil {
		return err
	}
	req.ResponseBody = io.NopCloser(bytes.NewReader(edited))
	return nil
}

func (ej *CmdEditJSON) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	draft := ej.Mgr.PeakRequestDraft(cmdCtx.ID())
	if draft == nil {
		return cmdCtx.Ctx, errors.New("cannot edit json, no request drafts")
	}

	draft.SetHeader("content-type", "application/json")
	return rawWfEdit(ej.BaseReqCmd, cmdCtx, util.EditJsonRawWf)
}

func (ex *CmdEditXml) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	draft := ex.Mgr.PeakRequestDraft(cmdCtx.ID())
	if draft == nil {
		return cmdCtx.Ctx, errors.New("cannot edit xml, no request drafts")
	}

	draft.SetHeader("content-type", "application/xml")
	return rawWfEdit(ex.BaseReqCmd, cmdCtx, util.EditXMLRawWf)
}

func (er *CmdEditReq) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	return er.SuggestWithoutParams(tokens)
}

func (er *CmdEditReq) AllowInModeWithoutArgs() bool {
	return false
}

func (er *CmdEditRespBody) AllowInModeWithoutArgs() bool {
	return false
}

func rawWfEdit(
	br *BaseReqCmd,
	cmdCtx *cmd.CmdCtx,
	fn util.RawEditWfFunc,
) (context.Context, error) {
	draft := br.Mgr.PeakRequestDraft(cmdCtx.ID())
	if draft == nil {
		return cmdCtx.Ctx, errors.New(
			"cannot edit json body, no request drafts",
		)
	}

	rawData, err := fn(config.GetAppCfg().GetDefaultEditor(), draft.Body)
	if err != nil {
		return cmdCtx.Ctx, err
	}

	draft.Body = string(rawData)

	return cmdCtx.Ctx, nil
}
