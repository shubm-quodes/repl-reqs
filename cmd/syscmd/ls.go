package syscmd

import (
	"context"
	"fmt"
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

func (ls *CmdLsVars) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	envMgr := config.GetEnvManager()
	envVars := envMgr.GetActiveEnvVars()

	if len(envVars) == 0 {
		fmt.Printf(
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

	fmt.Printf(
		"\n📦 Variables - (In currently active environment: '%s')\n\n",
		envMgr.GetActiveEnvName(),
	)
	for i, name := range keys {
		value := envVars[name]
		fmt.Printf("%d. %s: %s\n", i+1, name, util.GetTruncatedStr(value))
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
