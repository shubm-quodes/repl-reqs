package syscmd

import (
	"context"
	"sort"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/config"
	"github.com/shubm-quodes/repl-reqs/util"
)

const (
	// Root level cmd
	CmdLsName = "$ls"

	// Sub cmds
	CmdLsVarsName      = "vars"
	CmdLsTasksName     = "tasks"
	CmdLsSequencesName = "sequences"
	CmdLsEnvName       = "envs"
)

type CmdLs struct {
	*cmd.BaseCmd
}

type CmdLsVars struct {
	*cmd.BaseNonModeCmd
}

type CmdLsTasks struct {
	*cmd.BaseNonModeCmd
}

type CmdLsSequences struct {
	*cmd.BaseNonModeCmd
}

type CmdLsEnvs struct {
	*cmd.BaseNonModeCmd
}

func (ls *CmdLsVars) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	envMgr := config.GetEnvManager()
	envVars := envMgr.GetActiveEnvVars()
	hdlr := ls.GetCmdHandler()

	if len(envVars) == 0 {
		hdlr.OutF(
			cmdCtx,
			"\nNo variables in the currently active env: '%s' 🫤\n\n",
			envMgr.GetActiveEnvName(),
		)
		return cmdCtx.Ctx, nil
	}

	keys := make([]string, 0, len(envVars))
	for name := range envVars {
		keys = append(keys, name)
	}

	sort.Strings(keys)

	hdlr.OutF(
		cmdCtx,
		"\n📦 Variables - (In currently active environment: '%s')\n\n",
		envMgr.GetActiveEnvName(),
	)
	for i, name := range keys {
		value := envVars[name]
		hdlr.OutF(cmdCtx, "%d. %s: %s\n", i+1, name, util.GetTruncatedStr(value))
	}

	return cmdCtx.Ctx, nil
}

func (ls *CmdLsTasks) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ls.GetCmdHandler().ListTasks()
	return cmdCtx.Ctx, nil
}

func (ls *CmdLsSequences) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	ls.GetCmdHandler().ListSequences()
	return cmdCtx.Ctx, nil
}

func (lsEnvs *CmdLsEnvs) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	hdlr := lsEnvs.GetCmdHandler()
	envs := config.GetEnvManager().ListEnvs()

	if len(envs) == 0 {
		hdlr.Out(cmdCtx, "\nno envs have been initialized yet :/\n")
		return cmdCtx.Ctx, nil
	}

	for i, e := range envs {
		hdlr.Out(cmdCtx, "\nEnvironments- \n")
		hdlr.OutF(cmdCtx, "%d.) %s\n", i+1, e)
	}

	return cmdCtx.Ctx, nil
}
