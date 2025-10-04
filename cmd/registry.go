package cmd

type CmdRegistry struct {
	cmds map[string]Cmd
}

func NewCmdRegistry() *CmdRegistry {
	return &CmdRegistry{
		cmds: make(map[string]Cmd),
	}
}

func (reg *CmdRegistry) RegisterCmd(c ...Cmd) {
	for _, command := range c {
		reg.cmds[command.Name()] = command
	}
}

func (reg *CmdRegistry) GetCmdByName(name string) (Cmd, bool) {
	c, ok := reg.cmds[name]
	return c, ok
}

func (reg *CmdRegistry) GetCmds() map[string]Cmd {
  return reg.cmds
}

func (reg *CmdRegistry) GetAllCmds() []Cmd {
	cmds := make([]Cmd, len(reg.cmds))

	for _, c := range reg.cmds {
		cmds = append(cmds, c)
	}

	return cmds
}
