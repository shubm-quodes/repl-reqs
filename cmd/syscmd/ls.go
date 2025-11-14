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
	*cmd.BaseCmd
}

type CmdLsTasks struct {
	*cmd.BaseCmd
}

type CmdLsSequences struct {
	*cmd.BaseCmd
}

func (ls *CmdLsVars) Execute(cmdCtx *cmd.CmdCtx) (context.Context, error) {
	envVars := config.GetEnvManager().GetActiveEnvVars()

	keys := make([]string, 0, len(envVars))
	for name := range envVars {
		keys = append(keys, name)
	}

	sort.Strings(keys)

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
