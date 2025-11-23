package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/shubm-quodes/repl-reqs/config"
)

const (
	CmdIsEqName        = "$is_eq"
	CmdPlayStepName    = "$play_step"
	CmdFinalizeRecName = "$finalize"
)

type CmdIsEq struct {
	*BaseCmd
}

type CmdPlayStep struct {
	*BaseCmd
}

type CmdFinalizeRec struct {
	*BaseCmd
}

func (eq *CmdIsEq) Execute(cmdCtx *CmdCtx) (context.Context, error) {
	ctx, tokens := cmdCtx.Ctx, cmdCtx.ExpandedTokens
	if len(tokens) < 2 {
		return ctx, fmt.Errorf("'%s' requires two comparable args", CmdIsEqName)
	}

	val1, val2 := tokens[0], tokens[1]
	if val1 != val2 {
		return ctx, fmt.Errorf("inequivalent values '%s' and '%s'", val1, val2)
	}
	return ctx, nil
}

func (sv *CmdFinalizeRec) Execute(cmdCtx *CmdCtx) (context.Context, error) {
	hdlr := sv.GetCmdHandler()
	rec := hdlr.GetCurrentModeCmd().(*CmdRec)
	seqName := rec.currSequenceName

	if err := hdlr.FinalizeSequence(seqName); err != nil {
		return cmdCtx.Ctx, fmt.Errorf("failed to save sequence '%s'", seqName)
	} else {
		rec.isFinalized = true
		fmt.Printf("sequence '%s' saved successfully! 👌🏼\n", seqName)
		hdlr.ExitCmdMode()
		return cmdCtx.Ctx, nil
	}
}

func (sv *CmdFinalizeRec) saveSequence(seq Sequence, name string) error {
	sequenceCfg, err := sv.loadPreExistingSequences()
	if err != nil {
		return err
	}

	sequenceCfg[name] = seq
	if bytes, err := json.Marshal(sequenceCfg); err != nil {
		return err
	} else {
		return os.WriteFile(sv.SequenceFilePath(), bytes, 0644)
	}
}

func (sv *CmdFinalizeRec) loadPreExistingSequences() (map[string]Sequence, error) {
	rawSeqCfg, err := os.ReadFile(sv.SequenceFilePath())
	if err != nil {
		return nil, err
	}

	var seqCfg map[string]Sequence
	if err := json.Unmarshal(rawSeqCfg, &seqCfg); err != nil {
		return nil, err
	}

	return seqCfg, nil
}

func (sv *CmdFinalizeRec) SequenceFilePath() string {
	cfg := config.GetAppCfg()
	return path.Join(cfg.DirPath(), "sequences.json")
}

