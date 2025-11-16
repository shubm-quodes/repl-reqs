package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	// Root cmd
	CmdRecName = "$rec"
)

type CmdRec struct {
	*BaseCmd
	isFinalized       bool
	isLiveModeEnabled bool
	currSequenceName  string
}

func (cr *CmdRec) updatePromptStep() {
	hdlr := cr.GetCmdHandler()
	seq, _ := hdlr.GetSequence(cr.currSequenceName)
	hdlr.SetPrompt(fmt.Sprintf("rec(%s) 🔴 step #%d", cr.currSequenceName, len(seq)+1), "")
}

func (cr *CmdRec) Execute(cmdCtx *CmdCtx) (context.Context, error) {
	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens
	if len(tokens) == 0 {
		return ctx, errors.New("please specify a name for this recording sequence")
	}

	hdlr := cr.GetCmdHandler()
	if hdlr.GetCurrentModeCmd() != cr {
		return ctx, cr.enableRecMode(tokens)
	}

	if err := cr.handleSequenceCmd(tokens); err != nil {
		return ctx, err
	}
	cr.updatePromptStep()
	return ctx, nil
}

func (cr *CmdRec) registerNewSequence(sequenceName string) error {
	hdlr := cr.GetCmdHandler()
	if err := hdlr.RegisterSequence(sequenceName); err != nil {
		return err
	} else {
		cr.currSequenceName = sequenceName
	}

	fmt.Printf("Registered new sequence '%s'\n", sequenceName)
	return nil
}

func (cr *CmdRec) enableRecMode(tokens []string) error {
	var sequenceName string
	if len(tokens) >= 2 && tokens[0] == "live" {
		cr.isLiveModeEnabled = true
		sequenceName = strings.Join(tokens[1:], " ")
	} else {
		cr.isLiveModeEnabled = false // If previously it was enabled in live mode.. this will take care of it.
		sequenceName = strings.Join(tokens, " ")
	}

	if err := cr.registerNewSequence(sequenceName); err != nil {
		return err
	}
	cr.GetCmdHandler().
		PushCmdMode(fmt.Sprintf("rec(%s) 🔴 step #1", strings.Join(tokens, " ")), cr, true)
	return nil
}

func (cr *CmdRec) handleSequenceCmd(tokens []string) error {
	hdlr := cr.GetCmdHandler()

	if cr.isLiveModeEnabled {
		if _, err := hdlr.HandleRootCmd(hdlr.GetDefaultCtx(), tokens); err != nil {
			return err
		}
	}

	return hdlr.SaveSequenceStep(cr.currSequenceName, &Step{
		Cmd: tokens,
	})
}

func (cr *CmdRec) AllowRootCmdsWhileInMode() bool {
	return true
}

func (cr *CmdRec) cleanup() {
	if cr.isFinalized {
		return
	}

	cr.GetCmdHandler().DiscardSequence(cr.currSequenceName)
	fmt.Printf("sequence '%s' was not finalized\n", cr.currSequenceName)
}
