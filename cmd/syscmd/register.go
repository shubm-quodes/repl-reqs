package syscmd

import "github.com/shubm-quodes/repl-reqs/cmd"

func RegisterCmds(reg *cmd.CmdRegistry) {
	s := &CmdSet{cmd.NewBaseCmd(CmdSetName, "")}
	s.AddSubCmd(&CmdEnv{cmd.NewBaseCmd(CmdEnvName, "")}).
		AddSubCmd(&CmdVar{cmd.NewBaseCmd(CmdVarName, "")}).
		AddSubCmd(&CmdURL{NewBaseReqCmd(CmdURLName)}).
		AddSubCmd(&CmdHeader{NewInModeBaseReqCmd(CmdHeaderName)}).
		AddSubCmd(&CmdMultiHeaders{BaseReqCmd: NewBaseReqCmd(CmdMultiHeadersName)}).
		AddSubCmd(&CmdMethod{NewBaseReqCmd(CmdMethodName)}).
		AddSubCmd(&CmdBody{NewBaseReqCmd(CmdBodyName)}).
		AddSubCmd(&CmdPrompt{cmd.NewBaseCmd(CmdPromptName, "")}).
		AddSubCmd(&CmdMascot{cmd.NewBaseCmd(CmdMascotName, "")}).
		AddSubCmd(&CmdQuery{NewInModeBaseReqCmd(CmdQueryName)})

	n := &draftReqCmd{NewBaseReqCmd(CmdDraftReqName)}

	send := &CmdSend{ReqCmd: NewReqCmd(CmdSendName, nil)}

	ls := &CmdLs{cmd.NewBaseCmd(CmdLsName, "")}
	ls.AddSubCmd(&CmdLsVars{cmd.NewBaseNonModeCmd(CmdLsVarsName, "")}).
		AddSubCmd(&CmdLsTasks{cmd.NewBaseNonModeCmd(CmdLsTasksName, "")}).
		AddSubCmd(&CmdLsSequences{cmd.NewBaseNonModeCmd(CmdLsSequencesName, "")}).
		AddSubCmd(&CmdLsEnvs{cmd.NewBaseNonModeCmd(CmdLsEnvName, "")})

	save := &CmdSave{&BaseReqCmd{BaseCmd: cmd.NewBaseCmd(CmdSaveName, "")}}
	dlt := &CmdDelete{BaseCmd: cmd.NewBaseCmd(CmdDeleteName, "")}
	dlt.AddSubCmd(&CmdDeleteVar{BaseCmd: cmd.NewBaseCmd(CmdDeleteVarName, "")}).
		AddSubCmd(&CmdDeleteSeq{BaseCmd: cmd.NewBaseCmd(CmdDeleteSeqName, "")})

	edit := &CmdEdit{&BaseReqCmd{BaseCmd: cmd.NewBaseCmd(CmdEditName, "")}}
	edit.AddSubCmd(&CmdEditReq{&BaseReqCmd{BaseCmd: cmd.NewBaseCmd(CmdEditReqName, "")}}).
		AddSubCmd(&CmdEditRespBody{&BaseReqCmd{BaseCmd: cmd.NewBaseCmd(CmdEditRespBodyName, "")}}).
		AddSubCmd(&CmdEditJSON{&BaseReqCmd{BaseCmd: cmd.NewBaseCmd(CmdEditJsonName, "")}}).
		AddSubCmd(&CmdEditXml{&BaseReqCmd{BaseCmd: cmd.NewBaseCmd(CmdEditXmlName, "")}})

	p := &CmdPoll{&BaseReqCmd{BaseCmd: cmd.NewBaseCmd(CmdPollName, "")}}

	reg.RegisterCmd(s, n, send, ls, save, dlt, edit, p)
}
