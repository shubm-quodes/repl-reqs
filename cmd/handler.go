package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/nodding-noddy/repl-reqs/config"
	"github.com/nodding-noddy/repl-reqs/util"
)

type CmdMode struct {
	CmdName string
	prompt  string
	Cmd     Cmd
}

type Step struct {
	name string
	cmd  []string
}

type Sequence []Step

type ListernerAction string
type KeyListener struct {
	key     rune
	handler readline.FuncKeypressHandler
}

type KeyListenerRegistry map[ListernerAction]*KeyListener

type CmdHandler struct {
	appCfg             *config.AppCfg
	cmdRegistry        *CmdRegistry
	listeners          KeyListenerRegistry
	rl                 *readline.Instance
	modes              []*CmdMode
	mu                 sync.Mutex
	pauseSuggestions   bool
	isRecording        bool
	pauseTimer         *time.Timer
	activeSequenceName string
	sequenceRegistry   map[string]Sequence
	defaultCtx         context.Context
	taskUpdates        chan TaskStatus
	tasks              map[string]*TaskStatus
	spinner            *spinner.Spinner
	currFgTaskId       string
	fgTaskIdChan       chan string
	bgTaskIdChan       chan string
}

func NewCmdHandler(
	appCfg *config.AppCfg,
	rlCfg *readline.Config,
	reg *CmdRegistry,
) (*CmdHandler, error) {
	defaultCtx := context.Background()
	defaultCtx = context.WithValue(defaultCtx, CmdCtxIdKey, uuid.NewString())
	cmh := &CmdHandler{
		defaultCtx:   defaultCtx,
		appCfg:       appCfg,
		modes:        make([]*CmdMode, 0),
		listeners:    make(KeyListenerRegistry),
		taskUpdates:  make(chan TaskStatus),
		fgTaskIdChan: make(chan string),
		bgTaskIdChan: make(chan string),
		tasks:        map[string]*TaskStatus{},
		cmdRegistry:  reg,
		spinner:      spinner.New(spinner.CharSets[14], 100*time.Millisecond),
	}

	rlCfg.KeyListeners = make(map[rune]readline.FuncKeypressHandler)
	rlCfg.KeyListeners[0x06] = func() bool {
		cmh.bgTaskIdChan <- cmh.currFgTaskId
		return false
	}

	rlCfg.AutoComplete = cmh

	if rl, err := readline.NewEx(rlCfg); err != nil {
		return nil, err
	} else {
		cmh.rl = rl
		return cmh, nil
	}
}

func newCmdCtx(ctx context.Context, tokens []string) *CmdCtx {
	return &CmdCtx{
		Ctx:       ctx,
		RawTokens: tokens,
	}
}

func (h *CmdHandler) GetCmdRegistry() *CmdRegistry {
	return h.cmdRegistry
}

func (h *CmdHandler) GetCurrentCmdMode() Cmd {
	if len(h.modes) != 0 {
		return h.modes[len(h.modes)-1].Cmd
	}
	return nil
}

func (h *CmdHandler) GetDefaultCtx() context.Context {
	return h.defaultCtx
}

func (h *CmdHandler) GetCmdByName(name string) Cmd {
	if cmd, found := h.cmdRegistry.GetCmdByName(name); found {
		return cmd
	}
	return nil
}

func (h *CmdHandler) GetUpdateChan() chan<- TaskStatus {
	return h.taskUpdates
}

func (h *CmdHandler) GetAppCfg() *config.AppCfg {
	return h.appCfg
}

func (h *CmdHandler) PushCmdMode(modeName string, cmd Cmd) {
	m := new(CmdMode)
	m.CmdName = modeName
	m.Cmd = cmd
	h.modes = append(h.modes, m)
	h.SetPrompt(modeName, " ")
}

func (h *CmdHandler) SetCurrentCmdMode(mode *CmdMode) {
	h.SetPrompt(mode.CmdName, " ")
}

func (h *CmdHandler) SuggestRootCmds(partial string) (suggst [][]rune, offset int) {
	offset = len(partial)
	criteria := util.MatchCriteria[Cmd]{
		Search:     partial,
		SuffixWith: " ",
		M:          h.cmdRegistry.cmds,
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

	remainingTkns, cmd := Walk(cmd, util.StrArrToRune(tokens))
	rmTks := len(remainingTkns) + 1/2
	args := tokens[len(tokens)-rmTks:]
	cmdCtx := newCmdCtx(ctx, args)
	cmdCtx.ExpandedTokens = args

	if asyncCmd, ok := cmd.(AsyncCmd); ok {
		return h.HandleAsyncCmd(ctx, asyncCmd, args)
	}
	return cmd.Execute(cmdCtx)
}

func (h *CmdHandler) HandleAsyncCmd(
	ctx context.Context,
	cmd AsyncCmd,
	tokens []string,
) (context.Context, error) {
	task := NewTaskStatus(TaskStatusInitiated)
	h.spinner.Start()
	h.taskUpdates <- *task

	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	h.currFgTaskId = task.id
	cmd.SetTaskStatus(task)

	go func() {
		defer h.spinner.Stop()

		cmdCtx := newCmdCtx(taskCtx, tokens)
		cmdCtx.ExpandedTokens = tokens

		cmd.ExecuteAsync(cmdCtx)
	}()

	return taskCtx, nil
}

func (h *CmdHandler) handleUpdateChanClose() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.currFgTaskId != "" {
		h.spinner.Stop()
	}
}

func (h *CmdHandler) handleTaskUpdate(statusUpdate TaskStatus) {
	h.mu.Lock()
	defer h.mu.Unlock()

	task := h.findOrCreateTask(statusUpdate.id)
	h.updateTaskStatus(task, statusUpdate)
	h.handleTaskCompletionOrError(task)

	if h.currFgTaskId != "" {
		h.updateSpinnerMsg(task)
	}
}

func (h *CmdHandler) findOrCreateTask(id string) *TaskStatus {
	task, exists := h.tasks[id]
	if !exists {
		task = &TaskStatus{
			id:        id,
			createdAt: time.Now(),
		}
		h.tasks[id] = task
	}
	return task
}

func (h *CmdHandler) updateTaskStatus(task *TaskStatus, update TaskStatus) {
	task.message = update.message
	task.error = update.error
	task.done = update.done
	task.result = update.result
	task.output = update.output
}

func (h *CmdHandler) handleTaskCompletionOrError(task *TaskStatus) {
	if task.done && h.currFgTaskId == task.id && task.error == nil {
		h.spinner.Stop()
		fmt.Println("✅ Task completed\n", task.output)
		h.rl.Refresh()
		return
	}

	if task.error != nil {
		h.spinner.Stop()
		fmt.Println("❌ Task failed\n")
		msg := task.error.Error()
		if task.output != "" {
			msg = task.output
		}
		color.HiRed(msg)
		h.rl.Refresh()
	}
}

func (h *CmdHandler) listenForTaskUpdates() {
	for {
		select {
		case statusUpdate, ok := <-h.taskUpdates:
			if !ok {
				h.handleUpdateChanClose()
				return
			}
			h.handleTaskUpdate(statusUpdate)

		case fgTaskId := <-h.fgTaskIdChan:
			h.bringTaskToFg(fgTaskId)

		case <-h.bgTaskIdChan:
			h.sendTaskToBg()
		}
	}
}

// func (h *CmdHandler) listenForTaskUpdates() {
// 	for {
// 		select {
// 		case statusUpdate, ok := <-h.taskUpdates:
// 			if !ok {
// 				h.mu.Lock()
// 				if h.currFgTaskId != "" {
// 					h.spinner.Stop()
// 				}
// 				h.mu.Unlock()
// 				return
// 			}
// 			h.mu.Lock()
// 			task, exists := h.tasks[statusUpdate.id]
// 			if !exists {
// 				task = &taskStatus{
// 					id:        statusUpdate.id,
// 					createdAt: time.Now(),
// 				}
// 				h.tasks[statusUpdate.id] = task
// 			}
//
// 			task.message = statusUpdate.message
// 			task.error = statusUpdate.error
// 			task.done = statusUpdate.done
// 			task.result = statusUpdate.result
// 			task.output = statusUpdate.output
//
// 			if task.done && h.currFgTaskId == task.id {
//         h.spinner.Stop()
// 				fmt.Println("✅ Task completed\n", task.output)
// 				h.rl.Refresh()
// 			}
//
// 			if task.error != nil {
// 				h.spinner.Stop()
// 				color.Red(task.error.Error())
// 				h.rl.Refresh()
// 			}
//
// 			if h.currFgTaskId != "" {
// 				h.updateSpinnerMsg(task)
// 			}
//
// 			h.mu.Unlock()
// 		case fgTaskId := <-h.fgTaskIdChan:
// 			h.bringTaskToFg(fgTaskId)
// 		case <-h.bgTaskIdChan:
// 			h.sendTaskToBg()
// 		}
// 	}
// }

func (h *CmdHandler) updateSpinnerMsg(ts *TaskStatus) {
	if ts.error != nil {
		h.spinner.Suffix = " " + ts.error.Error()
	} else {
		h.spinner.Suffix = " " + ts.message
	}
}

func (h *CmdHandler) sendTaskToBg() {
	taskId := h.currFgTaskId
	if taskId != "" && h.spinner.Active() {
		h.currFgTaskId = ""
		h.spinner.Stop()
		fmt.Printf("task '%s' sent to background\n", taskId)
		h.rl.Refresh()
	}
}

func (h *CmdHandler) bringTaskToFg(taskId string) {
	h.mu.Lock()

	if h.currFgTaskId != "" {
		h.spinner.Stop()
	}

	h.currFgTaskId = taskId

	if task, exists := h.tasks[taskId]; exists {
		h.updateSpinnerMsg(task)
		h.currFgTaskId = taskId
		h.spinner.Start()
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
	h.rl.SetPrompt(FormatPrompt(newPrompt, mascot))
}

func (h *CmdHandler) RefreshPrompt() {
	h.rl.Refresh()
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
	if len(h.modes) != 0 {
		h.SetCurrentCmdMode(h.modes[len(h.modes)-1])
	} else {
		h.SetPrompt(h.appCfg.GetPrompt(), h.appCfg.GetPromptMascot())
	}
	m.Cmd.cleanup()
	return
}

func (h *CmdHandler) repl() {
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

func (h *CmdHandler) injectIntoCmds(reg map[string]Cmd) {
	for _, cmd := range reg {
		cmd.setHandler(h)
		subCmds := cmd.GetSubCmds()
		if len(subCmds) > 0 {
			h.injectIntoCmds(subCmds)
		}
	}
}

func (h *CmdHandler) activateListeners() {
	for _, lsnr := range h.listeners {
		rlCfg := h.rl.Config
		if rlCfg.KeyListeners == nil {
			rlCfg.KeyListeners = make(map[rune]readline.FuncKeypressHandler)
		}
		rlCfg.KeyListeners[lsnr.key] = lsnr.handler
	}
}

func (h *CmdHandler) Bootstrap() {
	h.injectIntoReg()
	h.activateListeners()
	h.repl()
}

func (h *CmdHandler) injectIntoReg() {
	if h.cmdRegistry == nil {
		panic("injection failed, handler registery not initialized")
	}
	h.injectIntoCmds(h.cmdRegistry.cmds)
}

func (h *CmdHandler) RegisterListener(
	key rune,
	action ListernerAction,
	fn readline.FuncKeypressHandler,
) {
	h.listeners[action] = &KeyListener{
		key:     key,
		handler: fn,
	}
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
