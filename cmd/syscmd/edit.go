package syscmd

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
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
	CmdEditRespBodyName = "response_body"
	CmdEditSeqName      = "sequence"
)

type CmdEdit struct {
	*BaseReqCmd
}

type CmdEditReq struct {
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

func (er *CmdEditRespBody) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
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
		return errors.New("content type header no found, failed to infer editor for response body")
	}

	respBytes, err := util.ReadAndResetIoCloser(&req.ResponseBody)
	if err != nil {
		log.Debug("failed to process response body for the last request: %w", err.Error())
		return errors.New("failed to process response body for the last request")
	}

	var respBody any
	if strings.ToLower(contentType[0]) == "application/json" {
		if err = json.Unmarshal(respBytes, &respBody); err != nil {
			log.Debug("failed to unmarshal json response %w", err)
			return errors.New("failed to unmarshal json response")
		}
		return util.EditJSON(&respBody, config.GetAppCfg().GetDefaultEditor())
	}

	if err = xml.Unmarshal(respBytes, &respBody); err != nil {
		log.Debug("failed to unmarshal xml response %w,", err)
		return errors.New("failed to unmarshal xml response")
	}

	return util.EditXML(&respBody, config.GetAppCfg().GetDefaultEditor())
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
