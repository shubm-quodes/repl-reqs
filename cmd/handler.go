package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
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

	GetSubCmds() SubCmd

	AddSubCmd(cmd Cmd) Cmd

	WalkTillLastSubCmd(tokens [][]rune) (remainingTkns [][]rune, c Cmd)

	filterSuggestions(partial string, offset int) [][]rune

	Execute(*CmdCtx) (context.Context, error)

	SetTaskStatus(*TaskStatus)

	GetTaskStatus() *TaskStatus

	cleanup()
}

type CmdMode struct {
	CmdName                  string
	prompt                   string
	Cmd                      Cmd
	AllowRootCmdsWhileInMode bool
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

func (h *ReplCmdHandler) SuggestRootCmds(partial string) ([][]rune, int) {
  offset := len(partial)
	criteria := &util.MatchCriteria[Cmd]{
		Search:     partial,
		SuffixWith: " ",
		M:          h.cmdRegistry.cmds,
	}

	if h.isRecordingModeActive {
		criteria.M = h.cmdRegistry.cmds
	}

	return util.GetMatchingMapKeysAsRunes(criteria), offset
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

func (h *ReplCmdHandler) IsCmdModeActive() bool {
	return len(h.modes) != 0
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
		return h.SuggestInModeCmds(mode, tokens)
	}

	return h.SuggestCmds(tokens)
}

func (h *ReplCmdHandler) SuggestInModeCmds(mode *CmdMode, tokens [][]rune) ([][]rune, int) {
	if mode == nil {
		return nil, 0
	}

	suggestions, offset := mode.Cmd.GetSuggestions(tokens)

	if mode.AllowRootCmdsWhileInMode {
		nativeSuggestions, _ := h.SuggestCmds(tokens)
		suggestions = append(suggestions, nativeSuggestions...)
	}

	return suggestions, offset
}

func (h *ReplCmdHandler) SuggestCmds(tokens [][]rune) ([][]rune, int) {
	partial := string(tokens[0])
	cmd := h.GetCmdByName(partial)
	if cmd == nil {
		sugg, offset := h.SuggestRootCmds(partial)
		return sugg, offset
	}

	subCmdTokens := tokens[1:]
	return cmd.GetSuggestions(subCmdTokens)
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

func (h *ReplCmdHandler) HandleCmd(
	ctx context.Context,
	tokens []string,
) (context.Context, error) {
	var cmd Cmd

	if modeCmd := h.GetCurrentModeCmd(); modeCmd != nil {
		cmd = modeCmd
	} else if cmd = h.GetCmdByName(tokens[0]); cmd != nil {
		tokens = tokens[1:]
	} else {
		return ctx, fmt.Errorf("invalid cmd '%s'"+"\n", tokens[0])
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

func (h *ReplCmdHandler) HandleAsyncCmd(
	ctx context.Context,
	cmd AsyncCmd,
	tokens []string,
) (context.Context, error) {
	task := h.NewTaskStatus(TaskStatusInitiated+" 🕙", cmd.GetFullyQualifiedName())
	h.spinner.Start()
	h.spinner.Suffix = task.message
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
	h.spinner.Stop()
	h.currFgTaskId = ""
	h.lastBgTaskId = ""
}

func (h *ReplCmdHandler) handleSuccessTaskStatus(task *TaskStatus) {
	h.resetTaskState()
	fmt.Println("✅ Task completed\n", task.output)
}

func (h *ReplCmdHandler) handleFailedTaskStatus(task *TaskStatus) {
	h.resetTaskState()

	fmt.Println("❌ Task failed\n")
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
	}
	h.rl.Refresh()
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
	fmt.Printf("%+v", taskIds)

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

	// if h.isRecording && err == nil {
	if h.isRecordingModeActive {
		seq := h.sequenceRegistry[h.activeSequenceName]
		seq = append(seq, Step{cmd: tokens})
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

func (h *ReplCmdHandler) SaveSequenceStep(seqName string, s Step) error {
	if seq, exists := h.sequenceRegistry[seqName]; exists {
		seq = append(seq, s)
		return nil
	} else {
		return fmt.Errorf("'%s' sequence doesn't exist", seqName)
	}
}

func (h *ReplCmdHandler) PlaySequence(name string) error {
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

func (h *ReplCmdHandler) SetPrompt(newPrompt string, mascot string) {
	h.rl.SetPrompt(FormatPrompt(newPrompt, mascot))
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
	h.injectIntoReg()
	h.activateListeners()
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
