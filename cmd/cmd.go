package cmd

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/nodding-noddy/repl-reqs/util"
)

var (
	cmdRegistry         = make(CmdRegistry)
	keyListenerRegistry = make(KeyListenerRegistry)
)

type Cmd interface {
	Name() string
	Desc() string
	setCmdHandler(*CmdHandler)
	GetCmdHandler() *CmdHandler
	GetSuggestions(tokens [][]rune) (suggestions [][]rune, offset int)
	GetSubCmds() SubCmd
	AddSubCmd(cmd Cmd) Cmd
	WalkTillLastSubCmd(tokens [][]rune) (remainingTkns [][]rune, c Cmd)
	filterSuggestions(partial string, offset int) [][]rune
	Execute(cmdContext context.Context, tokens []string) (context.Context, error)
	cleanup()
}

type BaseCmd struct {
	Name_   string
	Desc_   string
	SubCmds map[string]Cmd
	handler *CmdHandler
}

type AsyncCmd interface {
	Cmd
	ExecuteAsync(cmdContext context.Context, tokens []string)
}

type SubCmd map[string]Cmd

type CmdRegistry map[string]Cmd

type KeyListener struct {
	action  string
	handler readline.FuncKeypressHandler
}

type KeyListenerRegistry map[rune]*KeyListener

type taskStatus struct {
	id        string
	ctx       context.Context
	message   string
	error     error
	done      bool
	result    any
	createdAt time.Time
}

type TaskStatus interface {
	GetID() string
	GetMessage() string
	GetErr() error
	GetResult() any

	SetID() string
	SetMessage() string
	SetErr() error
	SetResult() any
}

func GetCmdByName(name string) (Cmd, bool) {
	cmd, exists := cmdRegistry[name]
	return cmd, exists
}

func (c *BaseCmd) Name() string {
	return c.Name_
}

func (c *BaseCmd) Desc() string {
	return c.Desc_
}

func (c *BaseCmd) GetSubCmds() SubCmd {
	return c.SubCmds
}

func (c *BaseCmd) GetCmdHandler() *CmdHandler {
	return c.handler
}

func (b *BaseCmd) AddSubCmd(cmd Cmd) Cmd {
	if b.SubCmds == nil {
		b.SubCmds = make(map[string]Cmd)
	}
	b.SubCmds[cmd.Name()] = cmd
	return b
}

func (b *BaseCmd) GetSubCmdList() []string {
	subCmds := make([]string, 0)
	for s := range b.SubCmds {
		subCmds = append(subCmds, s)
	}
	return subCmds
}

func Walk(cmd Cmd, tokens [][]rune) (remainingTkns [][]rune, finalCmd Cmd) {
	if len(tokens) == 0 {
		return nil, cmd
	}

	subCmds := cmd.GetSubCmds()
	if subCmds == nil {
		return tokens, cmd
	}

	subCmdName := string(tokens[0])
	if subCmd, ok := subCmds[subCmdName]; ok {
		return Walk(subCmd, tokens[1:])
	}

	return tokens, cmd
}

func (c *BaseCmd) WalkTillLastSubCmd(
	tokens [][]rune,
) (remainingTkns [][]rune, lastCmd Cmd) {
	if len(tokens) == 0 || c.SubCmds == nil {
		return tokens, c
	}

	firstToken := string(tokens[0])
	subCmd, ok := c.SubCmds[firstToken]
	if !ok || subCmd == nil {
		return tokens, c
	}

	return subCmd.WalkTillLastSubCmd(tokens[1:])
}

func (c *BaseCmd) filterSuggestions(partial string, offset int) [][]rune {
	if c == nil || c.SubCmds == nil {
		return nil
	}

	var suggestions [][]rune
	for name := range c.SubCmds {
		if strings.HasPrefix(name, partial) {
			suggestions = append(suggestions, []rune(name[offset:]+" "))
		}
	}
	return suggestions
}

func (c *BaseCmd) setCmdHandler(cmh *CmdHandler) {
	c.handler = cmh
}

func (c *BaseCmd) GetSuggestions(tokens [][]rune) (suggestions [][]rune, offset int) {
	remainingTkns, lastSubCmd := c.WalkTillLastSubCmd(tokens)
	if lastSubCmd == nil {
		return
	}

	if len(remainingTkns) > 1 {
		return nil, -1
	}

	search := ""
	if len(remainingTkns) == 1 {
		search = string(remainingTkns[0])
	}

	offset = len(search)
	return lastSubCmd.filterSuggestions(search, offset), offset
}

// Just a default Execute method if no args or an invalid sub cmd gets provided
func (c *BaseCmd) Execute(ctx context.Context, tokens []string) (context.Context, error) {
	subCmds := c.GetSubCmdList()
	formattedList := util.GetTruncatedStr(strings.Join(subCmds, ", "))
	fmt.Printf("available sub commands for %s are %v\n", c.Name_, formattedList)
	return ctx, nil
}

func (c *BaseCmd) cleanup() {}

func RegisterSysCmd(cmd Cmd) {
	name := strings.Trim(cmd.Name(), " ") // IKR.. :P
	if name == "" {
		typeName := reflect.TypeOf(cmd).Name()
		panic(
			fmt.Sprintf(
				"Failed to register system command '%s':"+
					"name cannot be empty!", typeName,
			),
		)
	}
	if !strings.HasPrefix(name, "$") {
		panic(
			fmt.Sprintf(
				"Failed to register system command with name '%s':"+
					"system command names should be prefixed with a '$' sign", name,
			),
		)
	}
	cmdRegistry[name] = cmd
}

func RegisterCmd(cmd AsyncCmd) {
	cmdRegistry[cmd.Name()] = cmd
}

func InjectCmdHandler(handler *CmdHandler) {
	injectHandlerInternal(cmdRegistry, handler)
}

func injectHandlerInternal(reg CmdRegistry, handler *CmdHandler) {
	for _, cmd := range reg {
		cmd.setCmdHandler(handler)
		subCmds := cmd.GetSubCmds()
		if len(subCmds) > 0 {
			injectHandlerInternal(CmdRegistry(subCmds), handler)
		}
	}
}
