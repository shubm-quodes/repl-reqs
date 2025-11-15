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
	isLiveModeEnabled bool
	currSequence      string
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

	return ctx, cr.handleSequenceCmd(tokens)
}

func (cr *CmdRec) registerNewSequence(sequenceName string) error {
	hdlr := cr.GetCmdHandler()
	if err := hdlr.RegisterSequence(sequenceName); err != nil {
		return err
	} else {
		cr.currSequence = sequenceName
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
	cr.GetCmdHandler().PushCmdMode(fmt.Sprintf("rec(%s) 🔴", strings.Join(tokens, " ")), cr, true)
	return nil
}

func (cr *CmdRec) handleSequenceCmd(tokens []string) error {
	hdlr := cr.GetCmdHandler()

	// Check for REC subcommands (like 'stop' or 'pause') first
	// if isRecSubcommand(tokens) { // You would need to implement this check
	//     // Handle REC subcommands here (or let them error out if not supported)
	// }

	if cr.isLiveModeEnabled {
		// 🚨 FIX: Call the method that executes the command as a root command,
		// bypassing the current command mode check.
		if _, err := hdlr.HandleRootCmd(hdlr.GetDefaultCtx(), tokens); err != nil {
			return err
		}
	}

	return hdlr.SaveSequenceStep(cr.currSequence, &Step{
		cmd: tokens,
	})
}

// func (cr *CmdRec) handleSequenceCmd(tokens []string) error {
// 	hdlr := cr.GetCmdHandler()
// 	if cr.isLiveModeEnabled {
// 		if _, err := hdlr.HandleCmd(hdlr.GetDefaultCtx(), tokens); err != nil {
// 			return err
// 		}
// 	}
//
// 	return hdlr.SaveSequenceStep(cr.currSequence, &Step{
// 		cmd: tokens,
// 	})
// }

func (cr *CmdRec) AllowRootCmdsWhileInMode() bool {
	return true
}
