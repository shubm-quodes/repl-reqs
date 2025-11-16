package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/shubm-quodes/repl-reqs/config"
)

const (
	CmdPlayName = "$play"

	SeqModeIndicatorKey = "isSeqEnabled"
	StepKey             = "step"
)

type SeqModeIndicator bool

type CmdPlay struct {
	*BaseCmd
}

func (pl *CmdPlay) ExecuteAsync(cmdCtx *CmdCtx) {
	hdlr := pl.GetCmdHandler()
	taskUpdate := hdlr.GetUpdateChan()
	taskStatus := pl.GetTaskStatus()
	tokens := cmdCtx.ExpandedTokens

	if len(tokens) == 0 {
		taskStatus.SetError(errors.New("please specify sequence name"))
		taskUpdate <- (*taskStatus)
		return
	}

	sequenceName := tokens[0]

	seq, err := hdlr.GetSequence(sequenceName)
	if err != nil {
		taskStatus.SetError(err)
		taskUpdate <- (*taskStatus)
		return
	}

	// --- Output Redirection ---

	originalStdout := os.Stdout

	var buf bytes.Buffer

	r, w, _ := os.Pipe()
	os.Stdout = w

	errChan := make(chan error)
	go func() {
		defer close(errChan)

		var execErr error
		seqCtx := context.Background()
		seqCtx = context.WithValue(seqCtx, SeqModeIndicatorKey, true)
		stepCtx := seqCtx
		for idx, step := range seq {
			stepCtx = context.WithValue(stepCtx, StepKey, step)
			var expandedCmd []string
			if expandedCmd, execErr = step.ExpandTokens(seq, config.GetEnvManager().GetActiveEnvVars()); execErr != nil {
				break
			}
			stepCtx, execErr = hdlr.HandleCmd(stepCtx, expandedCmd)
			if execErr != nil {
				break
			}

			taskStatus.SetMessage(fmt.Sprintf("step '%d' completed", idx+1))
			taskUpdate <- (*taskStatus)
		}
		errChan <- execErr
	}()

	go func() {
		io.Copy(&buf, r)
	}()

	execErr := <-errChan

	w.Close()
	os.Stdout = originalStdout

	if execErr != nil {
		fmt.Fprint(os.Stderr, buf.String())
		taskStatus.SetError(fmt.Errorf("sequence '%s' failed at step: %w", sequenceName, execErr))
		taskUpdate <- (*taskStatus)
		return
	}

	taskStatus.SetOutput(fmt.Sprintf("sequence '%s' successfully completed\n", sequenceName))
	taskStatus.SetDone(true)
	taskUpdate <- (*taskStatus)
}

