package main

type canaryConf struct {
	license         string
	agentVersion    string
	platform        string
	ansiblePassword string
}

func (c canaryConf) shouldProvisionLinux() bool {
	return c.platform == linux || c.platform == "all"
}

func (c canaryConf) shouldProvisionWindows() bool {
	return c.platform == windows || c.platform == "all"
}

func (c canaryConf) shouldProvisionMacos() bool {
	return c.platform == windows || c.platform == "all"
}
