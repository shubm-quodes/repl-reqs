package syscmd

import "github.com/shubm-quodes/repl-reqs/cmd"

func RegisterCmds(reg *cmd.CmdRegistry) {
	s := &setCmd{cmd.NewBaseCmd(CmdSetName, "")}
	s.AddSubCmd(&CmdEnv{cmd.NewBaseCmd(CmdEnvName, "")}).
		AddSubCmd(&CmdVar{cmd.NewBaseCmd(CmdVarName, "")}).
		AddSubCmd(&CmdURL{NewBaseReqCmd(CmdURLName)}).
		AddSubCmd(&CmdHeader{NewBaseReqCmd(CmdHeaderName)}).
		AddSubCmd(&CmdMultiHeaders{BaseReqCmd: NewBaseReqCmd(CmdMultiHeadersName)}).
		AddSubCmd(&CmdHTTPVerb{NewBaseReqCmd(CmdHTTPVerbName)}).
		AddSubCmd(&CmdPayload{NewBaseReqCmd(CmdPayloadName)}).
		AddSubCmd(&CmdPrompt{cmd.NewBaseCmd(CmdPromptName, "")}).
		AddSubCmd(&CmdMascot{cmd.NewBaseCmd(CmdMascotName, "")}).
		AddSubCmd(&CmdQuery{NewBaseReqCmd(CmdQueryName)})

	n := &draftReqCmd{NewBaseReqCmd(CmdDraftReqName)}
	send := &CmdSend{ReqCmd: NewReqCmd(CmdSendName, nil)}

	ls := &CmdLs{cmd.NewBaseCmd(CmdLsName, "")}
	ls.AddSubCmd(&CmdLsVars{cmd.NewBaseCmd(CmdLsVarsName, "")}).
		AddSubCmd(&CmdLsTasks{cmd.NewBaseCmd(CmdLsTasksName, "")})

	save := &CmdSave{&BaseReqCmd{BaseCmd: cmd.NewBaseCmd(CmdSaveName, "")}}
	reg.RegisterCmd(s, n, send, ls, save)
}
