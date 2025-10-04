package native

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nodding-noddy/repl-reqs/cmd"
	"github.com/nodding-noddy/repl-reqs/util"
)

const (
	CmdRecName = "$rec"

	//sub cmds
	CmdSaveRecName = "$save"
)

var sequenceRegistry = make(SequenceRegistry)
var recCmdSubCmdRegistery = make(cmd.SubCmd)

type Step struct {
	name string
	cmd  []string
}

type Sequence []Step

type SequenceRegistry map[string]Sequence

type CmdRec struct {
	*cmd.BaseCmd
	activeSequenceName string
}

type CmdSaveRec struct {
	*cmd.BaseCmd
}

func init() {
	r := new(CmdRec)
	r.Name_ = CmdRecName

	save := new(CmdSaveRec)
	save.Name_ = CmdSaveRecName
	recCmdSubCmdRegistery["$save"] = save
}

func (r *CmdRec) Execute(cmdCtx *cmd.CmdContext) error {
  tokens := cmdCtx.ExpandedTokens
	if len(tokens) == 0 {
		return errors.New("please specify sequence name")
	}

	if r.activeSequenceName == "" {
		name := tokens[0]
		sequenceRegistry[name] = make(Sequence, 0)
		return nil
	}

	return r.pushStep(tokens)
}

func (r *CmdRec) pushStep(tokens []string) error {
	if len(tokens) == 0 {
		return errors.New("step cannot be empty, please specify a command")
	}

	if _, cmd := r.WalkTillLastSubCmd(util.StrArrToRune(tokens)); cmd == nil {
		return fmt.Errorf("invalid command '%s'\n", strings.Join(tokens, " "))
	}

	n := r.activeSequenceName
	if s, exists := sequenceRegistry[n]; exists {
		s = append(s, Step{cmd: tokens})
	} else {
		fmt.Errorf("no active sequences found")
	}
	return nil
}
