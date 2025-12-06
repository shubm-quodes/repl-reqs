package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/shubm-quodes/repl-reqs/config"
	"github.com/shubm-quodes/repl-reqs/util"
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
	task := cmdCtx.Task
	tokens := cmdCtx.ExpandedTokens
	stepUChan := make(chan TaskStatus, 1)

	if len(tokens) == 0 {
		task.Fail(errors.New("please specify sequence name"))
		return
	}

	sequenceName := tokens[0]

	seq, err := hdlr.GetSequence(sequenceName)
	if err != nil {
		task.Fail(err)
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
		seq = pl.cloneSequence(seq)
		for idx, step := range seq {
			step.uChan = stepUChan
			step.Task = NewTask(
				fmt.Sprintf("%v #step", idx),
				strings.Join(step.Cmd, " "),
				stepUChan,
			)
			step.sequenceErrChan = errChan
			stepCtx = context.WithValue(stepCtx, StepKey, step)
			var expandedCmd []string
			if expandedCmd, execErr = step.ExpandTokens(seq, config.GetEnvManager().GetActiveEnvVars()); execErr != nil {
				break
			}

			task.UpdateMessage(
				fmt.Sprintf(
					"step %d: %s",
					idx+1,
					util.GetTruncatedStr(strings.Join(expandedCmd, " ")),
				),
			)

			if idx > 0 {
				step.ParentStep = seq[idx-1]
			}

			stepCtx, execErr = hdlr.HandleCmd(stepCtx, expandedCmd)
			if execErr != nil || step.HasFailed { // Has failed checks for async cmds
				break
			}
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
		task.Fail(
			fmt.Errorf("sequence '%s' failed at step: %w", sequenceName, execErr),
		)
		return
	}

	task.CompleteWithMessage(
		fmt.Sprintf("sequence '%s' successfully completed\n", sequenceName),
		nil,
	)
}

func (pl *CmdPlay) cloneSequence(originalSeq []*Step) []*Step {
	clonedSeq := make([]*Step, len(originalSeq))

	for idx, step := range originalSeq {
		clonedSeq[idx] = pl.cloneStep(step)
	}
	return clonedSeq
}

func (pl *CmdPlay) cloneStep(originalStep *Step) *Step {
	return &Step{
		Name: originalStep.Name,
		Cmd:  originalStep.Cmd,
	}
}

func (pl *CmdPlay) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	var (
		alreadySuggested [][]rune
		search           string
	)

	hdlr := pl.GetCmdHandler()
	if len(tokens) >= 1 {
		lastToken := string(tokens[len(tokens)-1])
		if _, err := hdlr.GetSequence(lastToken); err != nil {
			search = lastToken
		} else {
			alreadySuggested = tokens[0:]
		}
	}

	suggestions := util.MapSlice(
		hdlr.SuggestSequences(search),
		func(elem []rune, idx int) []rune { return util.TrimRunes(elem) },
	)

	return util.RuneSliceDiff(suggestions, alreadySuggested), len(search)
}
