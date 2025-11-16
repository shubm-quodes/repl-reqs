package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/shubm-quodes/repl-reqs/config"
	"github.com/shubm-quodes/repl-reqs/util"
)

type Cmd interface {
	Name() string

	Desc() string

	GetFullyQualifiedName() string

	setHandler(CmdHandler)

	SetParent(Cmd)

	GetCmdHandler() CmdHandler

	GetSuggestions(tokens [][]rune) (suggestions [][]rune, offset int)

	GetInModeSuggestions(tokens [][]rune) (suggestions [][]rune, offset int)

	GetSubCmds() SubCmd

	GetInModeCmds() SubCmd

	AddSubCmd(cmd Cmd) Cmd

	AddInModeCmd(cmd Cmd) Cmd

	WalkTillLastSubCmd(subCmdMap SubCmd, tokens [][]rune) (remainingTkns [][]rune, c Cmd)

	filterSuggestions(partial string, offset int) [][]rune

	Execute(*CmdCtx) (context.Context, error)

	SetTaskStatus(*TaskStatus)

	GetTaskStatus() *TaskStatus

	cleanup()
}

type AsyncCmd interface {
	Cmd

	ExecuteAsync(*CmdCtx)
}

type SetupAbleCmd interface {
	Setup(*ReplCmdHandler) error
}

type CmdMode struct {
	CmdName                  string
	prompt                   string
	Cmd                      Cmd
	AllowRootCmdsWhileInMode bool
}

type Sequence []*Step

type ListernerAction string
type KeyListener struct {
	key     rune
	handler readline.FuncKeypressHandler
}

type KeyListenerRegistry map[ListernerAction]*KeyListener

type ReplCmdHandler struct {
	appCfg                *config.AppCfg
	cmdRegistry           *CmdRegistry
	natvieCmdRegistry     *CmdRegistry
	listeners             KeyListenerRegistry
	rl                    *readline.Instance
	modes                 []*CmdMode
	mu                    sync.Mutex
	pauseSuggestions      bool
	isRecordingModeActive bool
	pauseTimer            *time.Timer
	activeSequenceName    string
	sequenceRegistry      map[string]Sequence
	defaultCtx            context.Context
	taskUpdates           chan TaskStatus
	tasks                 map[string]*TaskStatus
	spinner               *spinner.Spinner
	currFgTaskId          string
	lastBgTaskId          string
	fgTaskIdChan          chan string
	bgTaskIdChan          chan string
}

func NewCmdHandler(
	appCfg *config.AppCfg,
	rlCfg *readline.Config,
	reg *CmdRegistry,
) (*ReplCmdHandler, error) {
	cmh := &ReplCmdHandler{
		defaultCtx:   context.WithValue(context.Background(), CmdCtxIdKey, uuid.NewString()),
		appCfg:       appCfg,
		modes:        make([]*CmdMode, 0),
		listeners:    make(KeyListenerRegistry),
		taskUpdates:  make(chan TaskStatus),
		fgTaskIdChan: make(chan string),
		bgTaskIdChan: make(chan string),
		tasks:        make(map[string]*TaskStatus),
		cmdRegistry:  reg,
		spinner:      spinner.New(spinner.CharSets[14], 100*time.Millisecond),
	}

	rlCfg.KeyListeners = make(map[rune]readline.FuncKeypressHandler)
	rlCfg.KeyListeners[0x06] = func() bool {
		if cmh.currFgTaskId == "" {
			cmh.AttemptToBringLastBgTaskToFg()
		} else {
			cmh.bgTaskIdChan <- cmh.currFgTaskId
		}
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

func (h *ReplCmdHandler) NewTaskStatus(message, cmd string) *TaskStatus {
	h.mu.Lock()
	defer h.mu.Unlock()

	taskId := fmt.Sprintf("#%d", len(h.tasks)+1)
	taskStatus := &TaskStatus{
		id:        taskId,
		message:   message,
		cmd:       cmd,
		createdAt: time.Now(),
	}

	h.tasks[taskId] = taskStatus
	return taskStatus
}

func (h *ReplCmdHandler) GetCmdRegistry() *CmdRegistry {
	return h.cmdRegistry
}

func (h *ReplCmdHandler) GetCurrentModeCmd() Cmd {
	if len(h.modes) != 0 {
		return h.modes[len(h.modes)-1].Cmd
	}
	return nil
}

func (h *ReplCmdHandler) GetCurrentCmdMode() *CmdMode {
	if len(h.modes) != 0 {
		return h.modes[len(h.modes)-1]
	}

	return nil
}

func (h *ReplCmdHandler) GetDefaultCtx() context.Context {
	return h.defaultCtx
}

func (h *ReplCmdHandler) GetCmdByName(name string) Cmd {
	if cmd, found := h.cmdRegistry.GetCmdByName(name); found {
		return cmd
	}
	return nil
}

func (h *ReplCmdHandler) GetCurrModesInModeCmdByName(name string) Cmd {
	if cmd := h.GetCurrentModeCmd(); cmd == nil {
		return nil
	} else if inModeCmds := cmd.GetInModeCmds(); inModeCmds == nil {
		return nil
	} else {
		return inModeCmds[name]
	}
}

func (h *ReplCmdHandler) GetUpdateChan() chan<- TaskStatus {
	return h.taskUpdates
}

func (h *ReplCmdHandler) GetAppCfg() *config.AppCfg {
	return h.appCfg
}

func (h *ReplCmdHandler) PushCmdMode(modeName string, cmd Cmd, allowRootCmds bool) {
	m := new(CmdMode)
	m.CmdName = modeName
	m.Cmd = cmd
	m.AllowRootCmdsWhileInMode = allowRootCmds
	h.modes = append(h.modes, m)
	h.SetPrompt(modeName, " ")
}

func (h *ReplCmdHandler) SetCurrentCmdMode(mode *CmdMode) {
	h.SetPrompt(mode.CmdName, " ")
}

func (h *ReplCmdHandler) suggestCmdsFromMap(cmdMap map[string]Cmd, partial string) ([][]rune, int) {
	offset := len(partial)
	criteria := &util.MatchCriteria[Cmd]{
		Search:     partial,
		SuffixWith: " ",
		M:          cmdMap,
	}

	return util.GetMatchingMapKeysAsRunes(criteria), offset
}

func (h *ReplCmdHandler) SuggestRootCmds(partial string) ([][]rune, int) {
	return h.suggestCmdsFromMap(h.cmdRegistry.cmds, partial)
}

func (h *ReplCmdHandler) IsCmdModeActive() bool {
	return len(h.modes) != 0
}

func (h *ReplCmdHandler) SuggestInModeRootCmds(partial string) ([][]rune, int) {
	currModeCmd := h.GetCurrentModeCmd()
	return h.suggestCmdsFromMap(currModeCmd.GetInModeCmds(), partial)
}

func (h *ReplCmdHandler) SuggestInModeCmds(tokens [][]rune) ([][]rune, int) {
	cmdMode := h.GetCurrentCmdMode()
	partial := string(tokens[0])
	var (
		suggestions [][]rune
		offset      int
	)

	if cmd := h.GetCurrModesInModeCmdByName(partial); cmd == nil && len(tokens) == 1 {
		suggestions, offset = h.SuggestInModeRootCmds(partial)
	} else if cmd != nil {
		return cmd.GetSuggestions(tokens[1:])
	}

	modeSubCmds, offset := cmdMode.Cmd.GetSuggestions(tokens)
	suggestions = append(suggestions, modeSubCmds...)

	if cmdMode.AllowRootCmdsWhileInMode {
		var rootCmdSuggestions [][]rune
		rootCmdSuggestions, offset = h.SuggestCmds(tokens)
		suggestions = append(suggestions, rootCmdSuggestions...)
	}

	return suggestions, offset
}

func (h *ReplCmdHandler) SuggestCmds(tokens [][]rune) ([][]rune, int) {
	partial := string(tokens[0])
	if cmd := h.GetCmdByName(partial); cmd == nil && len(tokens) == 1 {
		return h.SuggestRootCmds(partial)
	} else if cmd != nil {
		subCmdTokens := tokens[1:]
		subCmdTokens, cmd = Walk(cmd, cmd.GetSubCmds(), subCmdTokens)
		return cmd.GetSuggestions(subCmdTokens)
	} else {
		return nil, 0
	}
}

func (h *ReplCmdHandler) SuggestVarNames(partial string) [][]rune {
	partial = strings.Trim(partial, " ")
	search := ""
	if len(partial) > 2 {
		search = partial[2:]
	}

	envMgr := config.GetEnvManager()
	return envMgr.GetMatchingVars(search)
}

func (h *ReplCmdHandler) SuggestSequences(partial string) [][]rune {
	criteria := &util.MatchCriteria[Sequence]{
		Search:     partial,
		SuffixWith: " ",
		M:          h.sequenceRegistry,
	}

	return util.GetMatchingMapKeysAsRunes(criteria)
}

func (h *ReplCmdHandler) Suggest(tokens [][]rune) ([][]rune, int) {
	if len(tokens) == 0 {
		return nil, 0
	}

	lastToken := string(tokens[len(tokens)-1])
	if isLikeAVariable(lastToken) {
		offset := len(lastToken[strings.LastIndex(lastToken, "{{"):]) - 2
		return h.SuggestVarNames(lastToken), offset
	}

	mode := h.GetCurrentCmdMode()
	if mode != nil {
		return h.SuggestInModeCmds(tokens)
	}

	return h.SuggestCmds(tokens)
}

func (h *ReplCmdHandler) PauseSuggestionsFor(d time.Duration) {
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

func (h *ReplCmdHandler) Do(line []rune, pos int) (suggestions [][]rune, offset int) {
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

func (h *ReplCmdHandler) GetRootCmd(tokens []string) (Cmd, []string, error) {
	if cmd := h.GetCurrentModeCmd(); cmd != nil {
		return cmd, tokens, nil
	}

	if len(tokens) == 0 {
		return nil, nil, fmt.Errorf("no command provided")
	}

	if cmd := h.GetCmdByName(tokens[0]); cmd != nil {
		return cmd, tokens[1:], nil
	}

	return nil, nil, fmt.Errorf("invalid command '%s'", tokens[0])
}

func (h *ReplCmdHandler) ResolveCommand(rootCmd Cmd, tokens []string) (Cmd, []string) {
	var (
		finalCmd      Cmd
		remainingTkns [][]rune
	)

	if h.GetCurrentModeCmd() != nil {
		remainingTkns, finalCmd = Walk(
			rootCmd,
			rootCmd.GetInModeCmds(),
			util.StrArrToRune(tokens),
		)
	} else {
		remainingTkns, finalCmd = Walk(
			rootCmd,
			rootCmd.GetSubCmds(),
			util.StrArrToRune(tokens),
		)
	}

	args := tokens[len(tokens)-len(remainingTkns):]

	return finalCmd, args
}

func (h *ReplCmdHandler) executeCommand(
	ctx context.Context,
	rootCmd Cmd,
	remainingTokens []string,
) (context.Context, error) {

	finalCmd, args := h.ResolveCommand(rootCmd, remainingTokens)

	cmdCtx := newCmdCtx(ctx, args)
	cmdCtx.ExpandedTokens = args

	if asyncCmd, ok := finalCmd.(AsyncCmd); ok {
		return h.HandleAsyncCmd(ctx, asyncCmd, args)
	}

	return finalCmd.Execute(cmdCtx)
}

func (h *ReplCmdHandler) HandleCmd(
	ctx context.Context,
	tokens []string,
) (context.Context, error) {
	rootCmd, remainingTokens, err := h.GetRootCmd(tokens)
	if err != nil {
		return ctx, err
	}

	return h.executeCommand(ctx, rootCmd, remainingTokens)
}

func (h *ReplCmdHandler) HandleRootCmd(
	ctx context.Context,
	tokens []string,
) (context.Context, error) {
	if len(tokens) == 0 {
		return ctx, errors.New("no command to execute")
	}

	cmd := h.GetCmdByName(tokens[0])
	if cmd == nil {
		return ctx, fmt.Errorf("invalid command '%s'", tokens[0])
	}

	return h.executeCommand(ctx, cmd, tokens[1:])
}

func (h *ReplCmdHandler) isSeqStepCtx(ctx context.Context) bool {
	is, ok := ctx.Value(SeqModeIndicatorKey).(bool)
	return ok && is
}

func (h *ReplCmdHandler) HandleAsyncSeqStep(cmd AsyncCmd, cmdCtx *CmdCtx) {
	ctx := cmdCtx.Ctx
	if step, ok := ctx.Value(StepKey).(*Step); ok {
		step.TaskStatus = cmd.GetTaskStatus()
	}

	cmd.ExecuteAsync(cmdCtx)
}

func (h *ReplCmdHandler) HandleAsyncCmd(
	ctx context.Context,
	cmd AsyncCmd,
	tokens []string,
) (context.Context, error) {
	task := h.NewTaskStatus(TaskStatusInitiated+" 🕙", cmd.GetFullyQualifiedName())
	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd.SetTaskStatus(task)

	cmdCtx := newCmdCtx(taskCtx, tokens)
	cmdCtx.ExpandedTokens = tokens

	if h.isSeqStepCtx(ctx) {
		h.HandleAsyncSeqStep(cmd, cmdCtx)
	} else {
		h.currFgTaskId = task.id
		h.spinner.Start()
		h.spinner.Suffix = task.message
		h.taskUpdates <- *task
		go func() {
			defer h.spinner.Stop()
			defer h.RefreshPrompt()

			cmd.ExecuteAsync(cmdCtx)
		}()
	}

	return taskCtx, nil
}

func (h *ReplCmdHandler) handleUpdateChanClose() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.currFgTaskId != "" {
		h.spinner.Stop()
	}
}

func (h *ReplCmdHandler) handleTaskUpdate(statusUpdate TaskStatus) {
	h.mu.Lock()
	defer h.mu.Unlock()

	task := h.findOrCreateTask(statusUpdate.id)
	h.updateTaskStatus(task, statusUpdate)
	h.handleTaskCompletionOrError(&statusUpdate)

	if h.currFgTaskId != "" {
		h.updateSpinnerMsg(task)
	}
}

func (h *ReplCmdHandler) findOrCreateTask(id string) *TaskStatus {
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

func (h *ReplCmdHandler) updateTaskStatus(task *TaskStatus, update TaskStatus) {
	task.message = update.message
	task.error = update.error
	task.done = update.done
	task.result = update.result
	task.output = update.output

	if !task.done && task.error == nil {
		h.spinner.Suffix = task.message
	}
}

func (h *ReplCmdHandler) resetTaskState() {
	h.currFgTaskId = ""
	h.lastBgTaskId = ""
}

func (h *ReplCmdHandler) handleSuccessTaskStatus(task *TaskStatus) {
	h.resetTaskState()
	fmt.Println("✅ Task completed\n", task.output)
}

func (h *ReplCmdHandler) handleFailedTaskStatus(task *TaskStatus) {
	h.resetTaskState()

	fmt.Println("❌ Task failed")
	fmt.Println()
	msg := task.error.Error()

	if task.output != "" {
		msg = task.output
	}

	color.HiRed(msg)
}

func (h *ReplCmdHandler) handleTaskCompletionOrError(task *TaskStatus) {
	if !task.done && task.error == nil {
		return
	}

	if h.currFgTaskId == task.id {
		if task.error == nil {
			h.handleSuccessTaskStatus(task)
		} else {
			h.handleFailedTaskStatus(task)
		}
		h.rl.Refresh()
	}
}

func (h *ReplCmdHandler) listenForTaskUpdates() {
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

		case h.lastBgTaskId = <-h.bgTaskIdChan:
			h.sendTaskToBg()
		}
	}
}

func (h *ReplCmdHandler) ListTasks() {
	if len(h.tasks) == 0 {
		fmt.Printf("nothing's running right now %s\n", "😴")
		return
	}

	taskIds := make([]string, 0, len(h.tasks))
	for id := range h.tasks {
		taskIds = append(taskIds, id)
	}

	sort.Strings(taskIds)

	fmt.Println("🕙 Tasks ~")
	for _, taskId := range taskIds {
		status := h.tasks[taskId]
		formatStr := "\n%s %s ~ %s"
		if status.error != nil {
			formatStr = formatStr + "❌"
		} else if status.done {
			formatStr = formatStr + "✅"
		} else {
			formatStr = formatStr + "In progres...🏃"
		}

		fmt.Printf(formatStr+"\n", status.id, status.cmd, status.output)
		fmt.Print("\n---------------------------------------------------\n")
	}
}

func (h *ReplCmdHandler) updateSpinnerMsg(ts *TaskStatus) {
	if ts.error != nil {
		h.spinner.Suffix = " " + ts.error.Error()
	} else {
		h.spinner.Suffix = " " + ts.message
	}
}

func (h *ReplCmdHandler) sendTaskToBg() {
	h.mu.Lock()
	defer h.mu.Unlock()

	taskId := h.currFgTaskId
	if taskId != "" && h.spinner.Active() {
		h.currFgTaskId = ""
		h.spinner.Stop()
		fmt.Printf("task '%s' sent to background\n", taskId)
		h.rl.Refresh()
	}
}

func (h *ReplCmdHandler) bringTaskToFg(taskId string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.currFgTaskId != "" {
		h.spinner.Stop()
	}

	h.currFgTaskId = taskId

	if task, exists := h.tasks[taskId]; exists {
		h.updateSpinnerMsg(task)
		h.currFgTaskId = taskId
		fmt.Println("\nlast active task is now in foreground")
		h.spinner.Start()
		h.RefreshPrompt()
	}
}

func (h *ReplCmdHandler) Handle(tokens []string) {
	if _, err := h.HandleCmd(h.defaultCtx, tokens); err != nil {
		fmt.Println(err.Error())
	}
}

func (h *ReplCmdHandler) tryRecordStep(tokens []string) {
	mode := h.GetCurrentCmdMode()
	if mode == nil {
		return
	}

	rec, ok := mode.Cmd.(*CmdRec)
	if !ok || !rec.isLiveModeEnabled {
		return
	}

	if len(tokens) > 0 && tokens[0] == CmdRecName {
		return
	}

	if err := h.SaveSequenceStep(rec.currSequenceName, &Step{
		Cmd: tokens,
	}); err != nil {
		fmt.Printf("Warning: Failed to save sequence step: %v\n", err)
	}
}

func (h *ReplCmdHandler) RegisterSequence(name string) error {
	if h.sequenceRegistry == nil {
		h.sequenceRegistry = make(map[string]Sequence)
	}

	if _, exists := h.sequenceRegistry[name]; exists {
		return errors.New("sequence '%s' already exists")
	}

	h.sequenceRegistry[name] = Sequence{}
	return nil
}

func (h *ReplCmdHandler) DiscardSequence(seqName string) error {
	if _, err := h.GetSequence(seqName); err != nil {
		return err
	} else {
		delete(h.sequenceRegistry, seqName)
		return h.refreshPersistedSequences()
	}
}

func (h *ReplCmdHandler) FinalizeSequence(seqName string) error {
	if seq, exists := h.sequenceRegistry[seqName]; exists {
		if len(seq) == 0 {
			return fmt.Errorf("cannot finalize sequence '%s', no steps were added", seqName)
		}
		return h.refreshPersistedSequences()
	} else {
		return fmt.Errorf("'%s' sequence doesn't exist", seqName)
	}
}

func (h *ReplCmdHandler) sequenceFilePath() string {
	cfg := config.GetAppCfg()
	return path.Join(cfg.DirPath(), "sequences.json")
}

func (h *ReplCmdHandler) loadSequences() error {
	rawSeqCfg, err := os.ReadFile(h.sequenceFilePath())
	if err != nil {
		return err
	}

	if err := json.Unmarshal(rawSeqCfg, &h.sequenceRegistry); err != nil {
		return err
	}
	return nil
}

func (h *ReplCmdHandler) refreshPersistedSequences() error {
	if bytes, err := json.MarshalIndent(h.sequenceRegistry, "", "  "); err != nil {
		return err
	} else {
		return os.WriteFile(h.sequenceFilePath(), bytes, 0644)
	}
}

func (h *ReplCmdHandler) SaveSequenceStep(seqName string, s *Step) error {
	if seq, exists := h.sequenceRegistry[seqName]; exists {
		if s.Name == "" {
			s.Name = fmt.Sprintf("step #%d", len(seq)+1)
		}
		seq = append(seq, s)
		h.sequenceRegistry[seqName] = seq
		return nil
	} else {
		return fmt.Errorf("'%s' sequence doesn't exist", seqName)
	}
}

func (h *ReplCmdHandler) GetSequence(name string) (Sequence, error) {
	seq, exists := h.sequenceRegistry[name]
	if !exists {
		return nil, fmt.Errorf("sequence '%s' not found", name)
	}

	return seq, nil
}

func (h *ReplCmdHandler) PlaySequence(name string) error {
	seq, exists := h.sequenceRegistry[name]
	if !exists {
		return fmt.Errorf("sequence '%s' not found", name)
	}

	ctx := context.Background()
	for _, s := range seq {
		h.HandleCmd(ctx, s.Cmd)
	}
	return nil
}

func (h *ReplCmdHandler) SetPrompt(newPrompt string, mascot string) {
	h.rl.SetPrompt(FormatPrompt(newPrompt, mascot))
}

func (h *ReplCmdHandler) SetIsRecMode(is bool) {
	h.isRecordingModeActive = is
}

func (h *ReplCmdHandler) RefreshPrompt() {
	h.rl.Refresh()
}

func (h *ReplCmdHandler) UpdatePromptEnv() {
	def := h.appCfg.GetPrompt()
	h.rl.SetPrompt(FormatPrompt(def, ""))
}

func (h *ReplCmdHandler) ExitCmdMode() (quitShell bool) {
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

func (h *ReplCmdHandler) repl() {
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

func (h *ReplCmdHandler) injectIntoCmds(reg map[string]Cmd) {
	for _, cmd := range reg {
		cmd.setHandler(h)
		subCmds := cmd.GetSubCmds()
		if len(subCmds) > 0 {
			h.injectIntoCmds(subCmds)
		}

		inModeCmds := cmd.GetInModeCmds()
		if len(inModeCmds) > 0 {
			h.injectIntoCmds(inModeCmds)
		}
	}
}

func (h *ReplCmdHandler) activateListeners() {
	for _, lsnr := range h.listeners {
		rlCfg := h.rl.Config
		if rlCfg.KeyListeners == nil {
			rlCfg.KeyListeners = make(map[rune]readline.FuncKeypressHandler)
		}
		rlCfg.KeyListeners[lsnr.key] = lsnr.handler
	}
}

func (h *ReplCmdHandler) Bootstrap() {
	h.registerNativeCmds()
	h.injectIntoReg()
	h.activateListeners()
	h.loadSequences()
	h.repl()
}

func (h *ReplCmdHandler) injectIntoReg() {
	if h.cmdRegistry == nil {
		panic("injection failed, handler registery not initialized")
	}
	h.injectIntoCmds(h.cmdRegistry.cmds)
}

func (h *ReplCmdHandler) RegisterListener(
	key rune,
	action ListernerAction,
	fn readline.FuncKeypressHandler,
) {
	h.listeners[action] = &KeyListener{
		key:     key,
		handler: fn,
	}
}

func (h *ReplCmdHandler) AttemptToBringLastBgTaskToFg() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.currFgTaskId != "" || h.lastBgTaskId == "" {
		return
	}
	h.fgTaskIdChan <- h.lastBgTaskId
}

func (h *ReplCmdHandler) GetDefaultCtxId() CmdCtxID {
	id, _ := h.defaultCtx.Value(CmdCtxIdKey).(CmdCtxID) // Yeah yeah don't worry.. there won't be a case where this is otherwise ^_^
	return id
}

func (h *ReplCmdHandler) registerNativeCmds() {
	rec := &CmdRec{BaseCmd: NewBaseCmd(CmdRecName, "")}

	rec.AddInModeCmd(&CmdIsEq{NewBaseCmd(CmdIsEqName, "")}).
		AddInModeCmd(&CmdPlayStep{NewBaseCmd(CmdPlayStepName, "")}).
		AddInModeCmd(&CmdFinalizeRec{NewBaseCmd(CmdFinalizeRecName, "")})

	play := &CmdPlay{BaseCmd: NewBaseCmd(CmdPlayName, "")}
	h.GetCmdRegistry().RegisterCmd(rec, play)
}

func isLikeAVariable(segment string) bool {
	return strings.LastIndex(segment, "{{") != -1
}

func prependSpc(options *[][]rune) {
	*options = make([][]rune, 1)
	(*options)[0] = []rune{32}
}

func FormatPrompt(promptTxt, mascot string) string {
	if strings.Trim(mascot, " ") == "" {
		mascot = config.GetDefaultMascot()
	}

	env := config.GetEnvManager().GetActiveEnvName()
	return fmt.Sprintf("%s (%s) %s>", promptTxt, env, mascot)
}
