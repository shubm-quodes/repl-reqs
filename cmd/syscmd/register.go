package syscmd

import "github.com/nodding-noddy/repl-reqs/cmd"

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

	reg.RegisterCmd(s, n, send)
}
