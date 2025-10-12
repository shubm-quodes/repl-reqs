package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/chzyer/readline"
)

const CmdCtxIdKey CmdCtxID = "cmdCtx"

var (
	keyListenerRegistry = make(KeyListenerRegistry)
)

type CmdCtxID string

type CmdCtx struct {
	Ctx            context.Context
	RawTokens      []string
	ExpandedTokens []string
}

type Cmd interface {
	Name() string
	Desc() string
	setHandler(*CmdHandler)
	GetCmdHandler() *CmdHandler
	GetSuggestions(tokens [][]rune) (suggestions [][]rune, offset int)
	GetSubCmds() SubCmd
	AddSubCmd(cmd Cmd) Cmd
	WalkTillLastSubCmd(tokens [][]rune) (remainingTkns [][]rune, c Cmd)
	filterSuggestions(partial string, offset int) [][]rune
	Execute(*CmdCtx) (context.Context, error)
	SetTaskStatus(*taskStatus)
	GetTaskStatus() *taskStatus
	cleanup()
}

type BaseCmd struct {
	Name_      string
	Desc_      string
	SubCmds    map[string]Cmd
	handler    *CmdHandler
	taskStatus *taskStatus
}

type AsyncCmd interface {
	Cmd
	ExecuteAsync(*CmdCtx)
}

type SubCmd map[string]Cmd

type KeyListener struct {
	action  string
	handler readline.FuncKeypressHandler
}

type KeyListenerRegistry map[rune]*KeyListener

func NewBaseCmd(name, desc string) *BaseCmd {
	return &BaseCmd{
		Name_: name,
		Desc_: desc,
	}
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

func (b *BaseCmd) GetTaskStatus() *taskStatus {
	return b.taskStatus
}

func (b *BaseCmd) SetTaskStatus(t *taskStatus) {
	b.taskStatus = t
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

func (c *BaseCmd) setHandler(cmh *CmdHandler) {
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
func (c *BaseCmd) Execute(cmdCtx *CmdCtx) (context.Context, error) {
	c.GetCmdHandler().PushCmdMode(c.Name_, c)
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

// parseParamsWithConditionalQuotes orchestrates the parameter parsing process.
func ParseCmdKeyValPairs(tokens []string) (map[string]string, error) {
	// Step 1: Recombine tokens that belong to a single quoted value
	recombinedTokens, err := recombineQuotedTokens(tokens)
	if err != nil {
		return nil, err
	}

	// Step 2: Parse the recombined tokens into a map
	return parseKeyValues(recombinedTokens)
}

// --- Token Recombination Logic (Complex State Machine) ---

// recombineQuotedTokens handles the logic of stitching together tokens that were
// split because a quoted value contained spaces.
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

// handleInsideQuoteToken processes a token when the parser is currently inside a quote.
func (s *state) handleInsideQuoteToken(token string) error {
	// If inside a quote, append the token with a space
	s.currentToken += " " + token

	// Check if this token ends the quote
	if len(token) > 0 && token[len(token)-1] == s.quoteChar {
		s.inQuote = false
		s.recombinedTokens = append(s.recombinedTokens, s.currentToken)
		s.currentToken = ""
	}
	return nil
}

// handleOutsideQuoteToken processes a token when the parser is NOT inside a quote.
func (s *state) handleOutsideQuoteToken(token string) error {
	// Check if this token starts a quoted parameter (key='value)
	if strings.ContainsRune(token, '=') {
		parts := strings.SplitN(token, "=", 2)
		value := parts[1]

		if len(value) > 0 {
			firstChar := value[0]
			// Check if it starts with a quote
			if firstChar == '\'' || firstChar == '"' {
				s.inQuote = true
				s.quoteChar = firstChar
				s.currentToken = token

				// Check if the quote is also closed in this same token
				if len(value) > 1 && value[len(value)-1] == s.quoteChar {
					s.inQuote = false
					s.recombinedTokens = append(s.recombinedTokens, s.currentToken)
					s.currentToken = ""
				}
				return nil // Handled the token, move to the next one
			}
		}
	}

	// If no quote started, or it was a non-quoted parameter, treat it as a complete token
	s.recombinedTokens = append(s.recombinedTokens, token)
	return nil
}

// --- Key-Value Parsing Logic ---

// parseKeyValues takes a slice of 'key=value' strings and converts them to a map,
// stripping quotes from values as needed.
func parseKeyValues(recombinedTokens []string) (map[string]string, error) {
	parsedParams := make(map[string]string)

	for _, token := range recombinedTokens {
		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			// Skip tokens that are not key=value pairs
			continue
		}

		key := parts[0]
		value := parts[1]

		// Strip quotes from the value
		value = stripQuotes(value)

		parsedParams[key] = value
	}

	return parsedParams, nil
}

// stripQuotes removes surrounding single or double quotes from a value string
// if they exist and match.
func stripQuotes(value string) string {
	if len(value) < 2 {
		return value
	}

	firstChar := value[0]
	lastChar := value[len(value)-1]

	// Check if the value is symmetrically quoted
	if (firstChar == '\'' && lastChar == '\'') || (firstChar == '"' && lastChar == '"') {
		// Strip the quotes
		return value[1 : len(value)-1]
	}

	return value
}

func (c *CmdCtx) ID() string {
	v := c.Ctx.Value(CmdCtxIdKey)
	id, _ := v.(string)
	return string(id)
}

// func parseParamsWithConditionalQuotes(tokens []string) (map[string]string, error) {
// 	parsedParams := make(map[string]string)
//
// 	// Step 1: Recombine tokens that belong to a single quoted value
// 	var recombinedTokens []string
// 	inQuote := false
// 	quoteChar := byte(0)
// 	currentToken := ""
//
// 	for _, token := range tokens {
// 		if inQuote {
// 			// If inside a quote, append the token with a space and check for closing quote
// 			currentToken += " " + token
//
// 			// Check if this token ends the quote
// 			if len(token) > 0 && token[len(token)-1] == quoteChar {
// 				inQuote = false
// 				recombinedTokens = append(recombinedTokens, currentToken)
// 				currentToken = ""
// 			}
// 		} else {
// 			// Not in a quote yet, check if this token starts one (key='value)
// 			if strings.ContainsRune(token, '=') {
// 				parts := strings.SplitN(token, "=", 2)
// 				value := parts[1]
//
// 				if len(value) > 0 {
// 					firstChar := value[0]
// 					// Check if it starts with a quote
// 					if firstChar == '\'' || firstChar == '"' {
// 						inQuote = true
// 						quoteChar = firstChar
// 						currentToken = token
//
// 						// Check if the quote is also closed in this same token
// 						if len(value) > 1 && value[len(value)-1] == quoteChar {
// 							inQuote = false
// 							recombinedTokens = append(recombinedTokens, currentToken)
// 							currentToken = ""
// 						}
// 						continue // Move to the next token
// 					}
// 				}
// 			}
//
// 			// If no quote started, or it was a non-quoted parameter, treat it as a complete token
// 			recombinedTokens = append(recombinedTokens, token)
// 		}
// 	}
//
// 	// Handle the case where the input ended with an unclosed quote
// 	if inQuote {
// 		return nil, fmt.Errorf("unclosed quote starting at: %s", currentToken)
// 	}
//
// 	// Step 2: Parse the recombined tokens
// 	for _, token := range recombinedTokens {
// 		parts := strings.SplitN(token, "=", 2)
// 		if len(parts) != 2 {
// 			// Skip tokens that are not key=value pairs (e.g., plain command arguments)
// 			continue
// 		}
//
// 		key := parts[0]
// 		value := parts[1]
//
// 		// Step 3: Remove surrounding quotes if they exist (based on your rule)
// 		if len(value) > 1 {
// 			firstChar := value[0]
// 			lastChar := value[len(value)-1]
//
// 			// If the value contains a space AND starts/ends with a quote, strip them.
// 			// The recombination logic already ensures it's a full value string.
// 			if (firstChar == '\'' && lastChar == '\'') || (firstChar == '"' && lastChar == '"') {
// 				// Strip the quotes
// 				value = value[1 : len(value)-1]
// 			}
// 		}
//
// 		parsedParams[key] = value
// 	}
//
// 	return parsedParams, nil
// }
