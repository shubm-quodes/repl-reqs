package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/chzyer/readline"
	"github.com/google/uuid"
	"github.com/nodding-noddy/repl-reqs/config"
	"github.com/nodding-noddy/repl-reqs/util"
)

type CmdMode struct {
	CmdName string
	Cmd     Cmd
}

type CmdContext struct {
	sync.Mutex
	data map[string]any
}

type Step struct {
	name string
	cmd  []string
}

type Sequence []Step

type CmdHandler struct {
	appCfg             *config.AppConfig
	rl                 *readline.Instance
	modes              []*CmdMode
	mu                 sync.Mutex
	pauseSuggestions   bool
	isRecording        bool
	pauseTimer         *time.Timer
	activeSequenceName string
	sequenceRegistry   map[string]Sequence
	defaultCtx         context.Context
	taskUpdates        chan taskStatus
	tasks              map[string]*taskStatus
}

func NewCmdHandler(appCfg *config.AppConfig, rlCfg *readline.Config) (*CmdHandler, error) {
	cmh := &CmdHandler{
		defaultCtx: context.Background(),
		appCfg:     appCfg,
		modes:      make([]*CmdMode, 0),
	}

	rlCfg.AutoComplete = cmh

	if rl, err := readline.NewEx(rlCfg); err != nil {
		return nil, err
	} else {
		cmh.rl = rl
		return cmh, nil
	}
}

func NewTaskStatus(message string) *taskStatus {
	return &taskStatus{
		id:        uuid.NewString(),
		message:   message,
		createdAt: time.Now(),
	}
}

func (h *CmdHandler) AssignShell(shell *readline.Instance) {
	h.rl = shell
}

func (h *CmdHandler) GetCurrentCmdMode() Cmd {
	if len(h.modes) != 0 {
		return h.modes[len(h.modes)-1].Cmd
	}
	return nil
}

func (h *CmdHandler) GetCmdByName(name string) Cmd {
	if cmd, found := cmdRegistry[name]; found {
		return cmd
	}
	return nil
}

func (h *CmdHandler) GetUpdateChan() chan<- taskStatus {
	return h.taskUpdates
}

func (h *CmdHandler) GetAppCfg() *config.AppConfig {
	return h.appCfg
}

func (h *CmdHandler) SetCurrentCmdMode(modeName string, cmd Cmd) {
	m := new(CmdMode)
	m.CmdName = modeName
	m.Cmd = cmd
	h.modes = append(h.modes, m)
	h.rl.SetPrompt(modeName + "> ")
}

func (h *CmdHandler) SuggestRootCmds(partial string) (suggst [][]rune, offset int) {
	offset = len(partial)
	criteria := util.MatchCriteria[Cmd]{
		Search:     partial,
		SuffixWith: " ",
		M:          cmdRegistry,
	}

	return util.GetMatchingMapKeysAsRunes(&criteria), offset

}

func (h *CmdHandler) SuggestVarNames(partial string) [][]rune {
	partial = strings.Trim(partial, " ")
	search := ""
	if len(partial) > 2 {
		search = partial[2:]
	}

	envMgr := config.GetEnvManager()
	return envMgr.GetMatchingVars(search)
}

func (h *CmdHandler) IsCmdModeActive() bool {
	return len(h.modes) != 0
}

func (h *CmdHandler) Suggest(tokens [][]rune) ([][]rune, int) {
	if len(tokens) == 0 {
		return nil, 0
	}

	lastToken := string(tokens[len(tokens)-1])
	if isLikeAVariable(lastToken) {
		return h.SuggestVarNames(lastToken), len(lastToken) - 2
	}

	mode := h.GetCurrentCmdMode()
	if mode != nil {
		return mode.GetSuggestions(tokens)
	}

	return h.SuggestCmds(tokens)
}

func (h *CmdHandler) SuggestCmds(tokens [][]rune) ([][]rune, int) {
	partial := string(tokens[0])
	cmd := h.GetCmdByName(partial)
	if cmd == nil {
		sugg, offset := h.SuggestRootCmds(partial)
		return sugg, offset
	}

	subCmdTokens := tokens[1:]
	return cmd.GetSuggestions(subCmdTokens)
}

func (h *CmdHandler) PauseSuggestionsFor(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pauseTimer != nil {
		h.pauseTimer.Stop()
	}

	h.pauseSuggestions = true

	h.pauseTimer = time.AfterFunc(d, func() {
		h.mu.Lock()
		h.pauseSuggestions = false
		h.pauseTimer = nil
		h.mu.Unlock()
	})
}

func (h *CmdHandler) Do(line []rune, pos int) (suggestions [][]rune, offset int) {
	if len(line) == 0 || h.pauseSuggestions {
		return nil, 0
	}

	trimmed := util.TrimRunes(line)
	tokens := util.TokenizeRunes(trimmed)
	if len(tokens) == 0 {
		return nil, 0
	}

	suggestions, offset = h.Suggest(tokens)

	leadingSpaceCount := len(line) - len(trimmed)
	if offset > 0 && leadingSpaceCount > 0 {
		return nil, 0
	}

	if pos == len(trimmed) && offset == 0 {
		prependSpc(&suggestions) // Ouch! Frankly I don't remember why I did this..
	}

	return suggestions, offset
}

// func (h *CmdHandler) HandleCmd(
// 	ctx context.Context,
// 	tokens []string,
// ) (context.Context, error) {
// 	if mode := h.GetCurrentCmdMode(); mode != nil {
// 		return mode.execute(ctx, tokens)
// 	}
//
// 	name := tokens[0]
// 	cmd := h.GetCmdByName(name)
// 	if cmd == nil {
// 		return ctx, fmt.Errorf("invalid cmd '%s'"+"\n", name)
// 	}
//
// 	return cmd.execute(ctx, tokens[1:])
// }

func (h *CmdHandler) HandleCmd(
	ctx context.Context,
	tokens []string,
) (context.Context, error) {
	var (
		cmd     Cmd
		cmdName string
	)

	if modeCmd := h.GetCurrentCmdMode(); modeCmd != nil {
		cmd = modeCmd
	} else {
		cmdName = tokens[0]
		cmd = h.GetCmdByName(cmdName)
		tokens = tokens[1:]
	}

	if cmd == nil {
		return ctx, fmt.Errorf("invalid cmd '%s'"+"\n", cmdName)
	}

	if asyncCmd, ok := cmd.(AsyncCmd); ok {
		return h.HandleAsyncCmd(ctx, asyncCmd, tokens)
	}

	remainingTkns, cmd := Walk(cmd, util.StrArrToRune(tokens))
	rmTks := len(remainingTkns) + 1/2
	return cmd.Execute(ctx, tokens[len(tokens)-rmTks:])

	// remainingTkns, cmd := cmd.WalkTillLastSubCmd(util.StrArrToRune(tokens))
	//  fmt.Printf("Type of the returned cmd: %T\n", cmd)
}

func (h *CmdHandler) HandleAsyncCmd(
	ctx context.Context,
	cmd AsyncCmd,
	tokens []string,
) (context.Context, error) {
	s := h.newSpinner()
	s.Start()
	defer s.Stop()

	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd.ExecuteAsync(taskCtx, tokens)
	return ctx, nil
}

func (h *CmdHandler) newSpinner() *spinner.Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Writer = os.Stderr
	s.Suffix = " Starting..."
	return s
}

func (h *CmdHandler) listenForTaskUpdates() {
	for statusUpdate := range h.taskUpdates {
		h.mu.Lock()
		task, exists := h.tasks[statusUpdate.id]
		if !exists {
			task = &taskStatus{
				id:        statusUpdate.id,
				createdAt: time.Now(),
			}
			h.tasks[statusUpdate.id] = task
		}

		task.message = statusUpdate.message
		task.error = statusUpdate.error
		task.done = statusUpdate.done
		task.result = statusUpdate.result

		h.mu.Unlock()
	}
}

func (h *CmdHandler) processTaskUpdates(
	ctx context.Context,
	updates <-chan taskStatus,
	s *spinner.Spinner,
) (context.Context, error) {
	for {
		select {
		case u, ok := <-updates:
			s.Suffix = " " + u.message
			if !ok {
				s.FinalMSG = "✅ Task completed successfully!\n"
				return ctx, nil
			}

			if u.done {
				if u.error != nil {
					s.FinalMSG = fmt.Sprintf("❌ Task failed: %v\n", u.error)
					return ctx, u.error
				}
				s.FinalMSG = "✅ Task completed successfully!\n"
				return ctx, nil
			}

			s.Suffix = " " + u.message

		case <-ctx.Done():
			s.FinalMSG = fmt.Sprintf("🛑 Task canceled: %v\n", ctx.Err())

			go func() {
				for range updates {
				}
			}()

			return ctx, ctx.Err()
		}
	}
}

func (h *CmdHandler) Handle(tokens []string) {
	if _, err := h.HandleCmd(h.defaultCtx, tokens); err != nil {
		fmt.Println(err.Error())
	}

	// if h.isRecording && err == nil {
	if h.isRecording {
		seq := h.sequenceRegistry[h.activeSequenceName]
		seq = append(seq, Step{cmd: tokens})
	}
}

func (h *CmdHandler) PlaySequence(name string) error {
	seq, exists := h.sequenceRegistry[name]
	if !exists {
		return fmt.Errorf("sequence '%s' not found", name)
	}

	ctx := context.Background()
	for _, s := range seq {
		h.HandleCmd(ctx, s.cmd)
	}
	return nil
}

func (h *CmdHandler) SetPrompt(newPrompt string, mascot string) {
	if strings.Trim(newPrompt, " ") == "" {
		// log.Warn("prompt cannot be empty, not changed.")
	}

	h.rl.SetPrompt(FormatPrompt(newPrompt, mascot))
}

func (h *CmdHandler) UpdatePromptEnv() {
  def := h.appCfg.GetPrompt()
  h.rl.SetPrompt(FormatPrompt(def, ""))
}

func (h *CmdHandler) ExitCmdMode() (quitShell bool) {
	if len(h.modes) == 0 {
		return true
	}
	m := h.modes[len(h.modes)-1]
	if m.CmdName == "" {
		return true
	}
	h.modes = h.modes[:len(h.modes)-1]
	m.Cmd.cleanup()
	return
}

func (h *CmdHandler) Repl(prompt string, mascot string) {
	if h.rl == nil {
		panic("shell not assigned on handler")
	}

	h.SetPrompt(h.appCfg.GetPrompt(), h.appCfg.GetPromptMascot())
	go h.listenForTaskUpdates()
	for {
		line, err := h.rl.Readline()

		if err == readline.ErrInterrupt {
			continue
		} else if err == io.EOF {
			quitShell := h.ExitCmdMode()
			if quitShell {
				fmt.Println("so long..👋")
				break
			}
		}
		line = strings.Trim(line, " ")
		if len(line) < 1 {
			continue
		}
		tokens := strings.Fields(line)
		h.Handle(tokens)
	}
}

func GetCmdContextVal[T any](ctx *CmdContext, key string) (T, bool) {
	ctx.Lock()
	defer ctx.Unlock()

	val, ok := ctx.data[key]
	if !ok {
		var zero T
		return zero, false
	}
	typedVal, ok := val.(T)
	if !ok {
		var zero T
		return zero, false
	}
	return typedVal, true
}

func SetCmdContextVal[T any](ctx *CmdContext, key string, value T) error {
	if key == "" {
		return errors.New("key cannot be an empty string")
	}

	ctx.Lock()
	defer ctx.Unlock()
	ctx.data[key] = value

	return nil
}

func isLikeAVariable(segment string) bool {
	return strings.HasPrefix(segment, "{{")
}

func prependSpc(options *[][]rune) {
	*options = make([][]rune, 1)
	(*options)[0] = []rune{32}
}

func FormatPrompt(promptTxt string, mascot string) string {
	if strings.Trim(mascot, " ") == "" {
		mascot = config.GetDefaultMascot()
	}

	env := config.GetEnvManager().GetActiveEnvName()
	return fmt.Sprintf("%s (%s) %s>", promptTxt, env, mascot)
}
