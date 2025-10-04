package syscmd

import "github.com/nodding-noddy/repl-reqs/cmd"

func RegisterCmds(reg *cmd.CmdRegistry) {
	s := &setCmd{&cmd.BaseCmd{Name_: CmdSetName}}
	s.AddSubCmd(&CmdEnv{&cmd.BaseCmd{Name_: CmdEnvName}}).
		AddSubCmd(&CmdVar{&cmd.BaseCmd{Name_: CmdVarName}}).
		AddSubCmd(&CmdURL{&cmd.BaseCmd{Name_: CmdURLName}}).
		AddSubCmd(&CmdHeader{&cmd.BaseCmd{Name_: CmdHeaderName}}).
		AddSubCmd(&CmdMultiHeaders{BaseCmd: &cmd.BaseCmd{Name_: CmdMultiHeadersName}}).
		AddSubCmd(&CmdHTTPVerb{&cmd.BaseCmd{Name_: CmdHTTPVerbName}}).
		AddSubCmd(&CmdPayload{&cmd.BaseCmd{Name_: CmdPayloadName}}).
		AddSubCmd(&CmdPrompt{&cmd.BaseCmd{Name_: CmdPromptName}}).
		AddSubCmd(&CmdMascot{&cmd.BaseCmd{Name_: CmdMascotName}})

	reg.RegisterCmd(s)
}
