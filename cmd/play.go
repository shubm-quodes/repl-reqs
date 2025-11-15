package cmd

import (
	"context"
	"errors"
	"fmt"
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

	// --- Output Redirection Setup ---

	// // Save the original os.Stdout
	// originalStdout := os.Stdout
	//
	// // Create an in-memory buffer to capture the output
	// var buf bytes.Buffer
	//
	// // Redirect os.Stdout to the buffer's writer
	// r, w, _ := os.Pipe()
	// os.Stdout = w

	// Run the sequence execution logic in a goroutine
	// The output from HandleCmd will be written to 'w' (our fake os.Stdout)
	errChan := make(chan error)
	go func() {
		defer close(errChan)

		var execErr error
		seqCtx := context.Background()
		seqCtx = context.WithValue(seqCtx, SeqModeIndicatorKey, true)
		stepCtx := seqCtx
		for idx, step := range seq {
			// Note: If hdlr.HandleCmd modifies 'ctx', you should update it here.
			stepCtx = context.WithValue(stepCtx, StepKey, step)
			stepCtx, execErr = hdlr.HandleCmd(stepCtx, step.cmd)
			if execErr != nil {
				break
			}

			var result string
			if step.TaskStatus != nil {
				if strVal, ok := step.TaskStatus.result.(string); ok {
					result = strVal
				}
			}
			taskStatus.SetMessage(fmt.Sprintf("step '%d' completed with result '%s'\n", idx+1, result))
			taskUpdate <- (*taskStatus)
		}
		errChan <- execErr
	}()

	// Read all output from the pipe into the buffer
	// We run this in parallel with the execution goroutine
	// go func() {
	// 	io.Copy(&buf, r)
	// }()

	// Wait for the execution goroutine to finish
	execErr := <-errChan

	// Close the writer end of the pipe and restore os.Stdout
	// w.Close()
	// os.Stdout = originalStdout

	// --- Output Redirection Teardown ---

	// 3. Check for errors and decide whether to print the buffered output.
	if execErr != nil {
		// An error occurred. Print the captured output for debugging/context.
		// fmt.Fprint(os.Stderr, buf.String()) // Print captured output to Stderr
		taskStatus.SetError(fmt.Errorf("sequence '%s' failed at step: %w", sequenceName, execErr))
		taskUpdate <- (*taskStatus)
		return
	}

	fmt.Println("sequence completed successfully")
	taskStatus.SetOutput(fmt.Sprintf("sequence '%s' successfully completed\n", sequenceName))
	taskStatus.SetDone(true)
	taskUpdate <- (*taskStatus)
}

// func (pl *CmdPlay) ExecuteAsync(cmdCtx *CmdCtx) {
// 	// func (pl *CmdPlay) Execute(cmdCtx *CmdCtx) (context.Context, error) {
// 	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens
// 	if len(tokens) == 0 {
// 		return ctx, errors.New("please specify sequence name")
// 	}
//
// 	hdlr := pl.GetCmdHandler()
// 	sequenceName := tokens[0]
//
// 	seq, err := hdlr.GetSequence(sequenceName)
// 	if err != nil {
// 		return ctx, err
// 	}
//
// 	// --- Output Redirection Setup ---
//
// 	// Save the original os.Stdout
// 	originalStdout := os.Stdout
//
// 	// Create an in-memory buffer to capture the output
// 	var buf bytes.Buffer
//
// 	// Redirect os.Stdout to the buffer's writer
// 	r, w, _ := os.Pipe()
// 	os.Stdout = w
//
// 	// Run the sequence execution logic in a goroutine
// 	// The output from HandleCmd will be written to 'w' (our fake os.Stdout)
// 	errChan := make(chan error)
// 	go func() {
// 		defer close(errChan)
//
// 		var execErr error
// 		seqCtx := context.Background()
// 		seqCtx = context.WithValue(seqCtx, SeqModeIndicatorKey, true)
// 		for _, step := range seq {
// 			// Note: If hdlr.HandleCmd modifies 'ctx', you should update it here.
// 			var stepCtx context.Context
// 			stepCtx, execErr = hdlr.HandleCmd(seqCtx, step.cmd)
// 			if execErr != nil {
// 				ctx = stepCtx // Update context if necessary before exiting loop
// 				break
// 			}
// 			ctx = stepCtx
// 		}
// 		errChan <- execErr
// 	}()
//
// 	// Read all output from the pipe into the buffer
// 	// We run this in parallel with the execution goroutine
// 	go func() {
// 		io.Copy(&buf, r)
// 	}()
//
// 	// Wait for the execution goroutine to finish
// 	execErr := <-errChan
//
// 	// Close the writer end of the pipe and restore os.Stdout
// 	w.Close()
// 	os.Stdout = originalStdout
//
// 	// --- Output Redirection Teardown ---
//
// 	// 3. Check for errors and decide whether to print the buffered output.
// 	if execErr != nil {
// 		// An error occurred. Print the captured output for debugging/context.
// 		fmt.Fprint(os.Stderr, buf.String()) // Print captured output to Stderr
// 		return ctx, fmt.Errorf("sequence '%s' failed at step: %w", sequenceName, execErr)
// 	}
//
// 	// 4. Success message (only if all steps completed).
// 	fmt.Printf("sequence '%s' successfully completed\n", sequenceName)
// 	return ctx, nil
// }

// func (pl *CmdPlay) Execute(cmdCtx *CmdCtx) (context.Context, error) {
// 	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens
// 	if len(tokens) == 0 {
// 		return ctx, errors.New("please specify sequence name")
// 	}
//
// 	hdlr := pl.GetCmdHandler()
// 	sequenceName := tokens[0]
// 	if seq, err := hdlr.GetSequence(sequenceName); err != nil {
// 		return ctx, err
// 	} else {
// 		for _, step := range seq {
// 			if ctx, err := hdlr.HandleCmd(hdlr.GetDefaultCtx(), step.cmd); err != nil {
// 				return ctx, err
// 			}
// 		}
// 	}
// 	fmt.Printf("sequence '%s' successfully completed\n", sequenceName)
// 	return ctx, nil
// }
