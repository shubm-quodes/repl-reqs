package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/shubm-quodes/repl-reqs/config"
)

const CmdCtxIdKey CmdCtxID = "cmdCtx"

type CmdCtxID string

type CmdCtx struct {
	Ctx            context.Context
	RawTokens      []string
	ExpandedTokens []string
}

type CmdHandler interface {
	GetCurrentModeCmd() Cmd

	GetCurrentCmdMode() *CmdMode

	GetSequence(name string) (Sequence, error)

	GetAppCfg() *config.AppCfg

	GetUpdateChan() chan<- TaskStatus

	GetCmdRegistry() *CmdRegistry

	GetDefaultCtx() context.Context

	SetPrompt(prompt, mascot string)

	SetIsRecMode(bool)

	SuggestCmds(tokens [][]rune) ([][]rune, int)

	SuggestSequences(partial string) [][]rune

	SuggestVarNames(partial string) [][]rune

	HandleRootCmd(ctx context.Context, tokens []string) (context.Context, error)

	HandleCmd(ctx context.Context, tokens []string) (context.Context, error)

	PushCmdMode(cm Cmd)

	ExitCmdMode() bool

	UpdatePromptEnv()

	ListTasks()

	ListSequences()

	RegisterSequence(sequenceName string) error

	SaveSequenceStep(sequenceName string, step *Step) error

	FinalizeSequence(name string) error

	DiscardSequence(name string) error

	Inject(c Cmd)
}

type BaseCmd struct {
	Name_      string
	Desc_      string
	SubCmds    SubCmd
	InModeCmds SubCmd
	parent     Cmd
	handler    CmdHandler
	taskStatus *TaskStatus
}

type BaseNonModeCmd struct {
	*BaseCmd
}

type SubCmd map[string]Cmd

func NewBaseCmd(name, desc string) *BaseCmd {
	return &BaseCmd{
		Name_: name,
		Desc_: desc,
	}
}

func NewBaseNonModeCmd(name, desc string) *BaseNonModeCmd {
	return &BaseNonModeCmd{
		&BaseCmd{
			Name_: name,
			Desc_: desc,
		},
	}
}

func (c *BaseCmd) Name() string {
	return c.Name_
}

func (c *BaseCmd) Desc() string {
	return c.Desc_
}

func (c *BaseCmd) GetFullyQualifiedName() string {
	if c.parent == nil {
		return c.Name()
	}

	return c.parent.GetFullyQualifiedName() + " " + c.Name()
}

func (c *BaseCmd) GetSubCmds() SubCmd {
	return c.SubCmds
}

func (c *BaseCmd) GetInModeCmds() SubCmd {
	return c.InModeCmds
}

func (c *BaseCmd) GetCmdHandler() CmdHandler {
	return c.handler
}

func (b *BaseCmd) GetSubCmdList() []string {
	subCmds := make([]string, 0)
	for s := range b.SubCmds {
		subCmds = append(subCmds, s)
	}
	return subCmds
}

func (b *BaseCmd) GetModeName() string {
	return b.Name()
}

func (b *BaseCmd) GetTaskStatus() *TaskStatus {
	return b.taskStatus
}

func (b *BaseCmd) SetTaskStatus(t *TaskStatus) {
	b.taskStatus = t
}

func (b *BaseCmd) SetParent(parent Cmd) {
	b.parent = parent
}

func (b *BaseCmd) AddSubCmd(cmd Cmd) Cmd {
	if b.SubCmds == nil {
		b.SubCmds = make(SubCmd)
	}
	b.SubCmds[cmd.Name()] = cmd
	cmd.SetParent(b)
	return b
}

func (b *BaseCmd) AddInModeCmd(cmd Cmd) Cmd {
	if b.InModeCmds == nil {
		b.InModeCmds = make(SubCmd)
	}

	b.InModeCmds[cmd.Name()] = cmd
	cmd.SetParent(b)
	return b
}

func (b *BaseCmd) AllowInModeWithoutArgs() bool {
	return true // By default allow in mode.
}

func (b *BaseNonModeCmd) AllowInModeWithoutArgs() bool {
	return false // By default allow in mode.
}

func (b *BaseCmd) AllowRootCmdsWhileInMode() bool {
	return false
}

func Walk(
	cmd Cmd,
	subCmdMap map[string]Cmd,
	tokens [][]rune,
) (remainingTkns [][]rune, finalCmd Cmd) {
	if len(tokens) == 0 {
		return nil, cmd
	}

	if subCmdMap == nil {
		return tokens, cmd
	}

	subCmdName := string(tokens[0])
	if subCmd, ok := subCmdMap[subCmdName]; ok {
		return Walk(subCmd, subCmdMap, tokens[1:])
	}

	return tokens, cmd
}

func (c *BaseCmd) WalkTillLastSubCmd(
	subCmdMap SubCmd,
	tokens [][]rune,
) (remainingTkns [][]rune, lastCmd Cmd) {
	if len(tokens) == 0 {
		return tokens, c
	}

	if len(subCmdMap) == 0 {
		return tokens, c
	}

	firstToken := string(tokens[0])
	subCmd, ok := subCmdMap[firstToken]

	if !ok || subCmd == nil {
		return tokens, c
	}

	if nextBaseCmd, isBase := subCmd.(*BaseCmd); isBase {
		return subCmd.WalkTillLastSubCmd(nextBaseCmd.SubCmds, tokens[1:])
	}

	return tokens[1:], subCmd
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

func (c *BaseCmd) setHandler(cmh CmdHandler) {
	c.handler = cmh
}

func (c *BaseCmd) GetSuggestions(tokens [][]rune) ([][]rune, int) {
	return c.getSuggestions(c.SubCmds, tokens)
}

func (c *BaseCmd) GetInModeSuggestions(tokens [][]rune) ([][]rune, int) {
	if len(tokens) == 0 {
		return nil, 0
	}

	return c.getSuggestions(c.InModeCmds, tokens)
}

func (c *BaseCmd) getSuggestions(
	subCmdMap SubCmd,
	tokens [][]rune,
) (suggestions [][]rune, offset int) {
	remainingTkns, lastSubCmd := c.WalkTillLastSubCmd(subCmdMap, tokens)

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
func (c *BaseCmd) Execute(cmdCtx *CmdCtx) (context.Context, error) {
	hdlr := c.GetCmdHandler()
	if hdlr.GetCurrentModeCmd() != c {
		hdlr.PushCmdMode(c)
	}
	return cmdCtx.Ctx, nil
}

func (c *BaseCmd) cleanup() {}

// state is a struct to hold the current parsing state for token recombination.
type state struct {
	recombinedTokens []string
	inQuote          bool
	quoteChar        byte
	currentToken     string
}

func ParseCmdKeyValPairs(tokens []string) (map[string]string, error) {
	recombinedTokens, err := recombineQuotedTokens(tokens)
	if err != nil {
		return nil, err
	}

	return parseKeyValues(recombinedTokens)
}

func recombineQuotedTokens(tokens []string) ([]string, error) {
	s := state{
		recombinedTokens: make([]string, 0),
		inQuote:          false,
		quoteChar:        byte(0),
		currentToken:     "",
	}

	for _, token := range tokens {
		var err error
		if s.inQuote {
			err = s.handleInsideQuoteToken(token)
		} else {
			err = s.handleOutsideQuoteToken(token)
		}

		if err != nil {
			return nil, err
		}
	}

	// Handle the case where the input ended with an unclosed quote
	if s.inQuote {
		return nil, fmt.Errorf("unclosed quote starting at: %s", s.currentToken)
	}

	return s.recombinedTokens, nil
}

func (s *state) handleInsideQuoteToken(token string) error {
	s.currentToken += " " + token

	if len(token) > 0 && token[len(token)-1] == s.quoteChar {
		s.inQuote = false
		s.recombinedTokens = append(s.recombinedTokens, s.currentToken)
		s.currentToken = ""
	}
	return nil
}

func (s *state) handleOutsideQuoteToken(token string) error {
	if strings.ContainsRune(token, '=') {
		parts := strings.SplitN(token, "=", 2)
		value := parts[1]

		if len(value) > 0 {
			firstChar := value[0]
			if firstChar == '\'' || firstChar == '"' {
				s.inQuote = true
				s.quoteChar = firstChar
				s.currentToken = token

				if len(value) > 1 && value[len(value)-1] == s.quoteChar {
					s.inQuote = false
					s.recombinedTokens = append(s.recombinedTokens, s.currentToken)
					s.currentToken = ""
				}
				return nil // Handled the token, move to the next one
			}
		}
	}

	s.recombinedTokens = append(s.recombinedTokens, token)
	return nil
}

func parseKeyValues(recombinedTokens []string) (map[string]string, error) {
	parsedParams := make(map[string]string)

	for _, token := range recombinedTokens {
		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		value = stripQuotes(value)

		parsedParams[key] = value
	}

	return parsedParams, nil
}

func stripQuotes(value string) string {
	if len(value) < 2 {
		return value
	}

	firstChar := value[0]
	lastChar := value[len(value)-1]

	if (firstChar == '\'' && lastChar == '\'') || (firstChar == '"' && lastChar == '"') {
		return value[1 : len(value)-1]
	}

	return value
}

func (c *CmdCtx) ID() string {
	v := c.Ctx.Value(CmdCtxIdKey)
	id, _ := v.(string)
	return string(id)
}
