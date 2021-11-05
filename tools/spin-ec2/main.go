package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/mod/semver"

	"log"
	"math/rand"
	"os"
	"os/user"
	"path"
	"time"
)

const (
	instancesFile        = "test/automated/ansible/group_vars/localhost/main.yml"
	inventoryForCreation = "test/automated/ansible/custom-instances.yml"
	inventoryLocal       = "test/automated/ansible/inventory.local"
	inventoryProvisioned = "test/automated/ansible/inventory.ec2"
	inventoryMacos       = "test/automated/ansible/inventory.macos.ec2"
	colorArm64           = "\033[32m"
	colorAmd64           = "\033[34m"
	colorReset           = "\033[0m"

	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

var (
	letterRunes       = []rune("abcdefghijklmnopqrstuvwxyz")
	hostPrefix        = "canary"
	isLicenseRequired = false
	skipVMCreation    = false
)

func main() {
	interactive := len(os.Args) == 1

	if interactive {
		interactiveMode()
		return
	}

	cliMode()
}

func interactiveMode() {

	skipVMCreationString := askUser(fmt.Sprintf("Do you want to skip VM provision  [(%s)es / (%s)o / (%s)uit]: [no] ", colorizeGreen("y"), colorizeYellow("n"), colorizeRed("q")))

	if skipVMCreationString == "yes" || skipVMCreationString == "y" {
		skipVMCreation = true
	}

	var provisionHostPrefix string
	if !skipVMCreation {
		provisionHostPrefix = createVMs()
	}

	fmt.Printf("\nPossible provision options\n")

	provisionOpts := newProvisionOptions()
	provisionOpts.print()

	var chosenProvisionOptions provisionOptions
	for {

		chosenProvisionInput := askUser(fmt.Sprintf("Choose an option : [%s] ", colorizeYellow("0")))

		if chosenProvisionInput == "" {
			chosenProvisionInput = "0"
		}

		chosenProvisionNumbers, err := stringToNumbers(chosenProvisionInput)
		if err != nil {
			fmt.Printf(colorizeRed(err.Error() + ". Please enter valid input\n"))
			continue
		}

		chosenProvisionOptions, err = provisionOpts.filter(chosenProvisionNumbers)
		if err != nil {
			fmt.Printf(colorizeRed(err.Error() + ". Please enter valid input\n"))
			continue
		}

		for _, cOpt := range chosenProvisionOptions {
			if cOpt.licenseKeyRequired {
				isLicenseRequired = true
			}
		}

		break
	}

	license := ""

	if isLicenseRequired {
		license = askUser("NR license key required for chosen provision option(s): ")
	}

	// ask for ansible_password  (just necessary for windows)
	// if it's empty it will not be used
	ansiblePassword := askUser("Insert ansible_password if needed to provision Windows hosts:")

	curPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	if !skipVMCreation {
		execNameArgs("ansible-playbook",
			"-i", path.Join(curPath, inventoryLocal),
			"--extra-vars", "@"+path.Join(curPath, inventoryForCreation),
			"-e", "instance_prefix="+provisionHostPrefix+":",
			path.Join(curPath, "test/automated/ansible/provision.yml"))

		execNameArgs("ansible-playbook",
			"-i", path.Join(curPath, inventoryProvisioned),
			path.Join(curPath, "test/automated/ansible/install-requirements.yml"))
	}

	if len(chosenProvisionOptions) > 0 {

		for _, chosenOpt := range chosenProvisionOptions {

			if chosenOpt.playbook == "" {
				continue
			}

			var arguments []string

			arguments = append(arguments, "-i", path.Join(curPath, inventoryProvisioned))

			if chosenOpt.renderArgs() != "" {
				arguments = append(arguments, chosenOpt.renderArgs())
			}

			arguments = append(arguments, "-e", "nr_license_key="+license)
			if ansiblePassword != "" {
				arguments = append(arguments, "-e", "ansible_password="+ansiblePassword)
			}

			arguments = append(arguments, path.Join(curPath, chosenOpt.playbook))

			execNameArgs("ansible-playbook", arguments...)
		}
	}
}

func createVMs() string {
	rand.Seed(time.Now().UnixNano())
	var err error

	ansibleGroupVars, err := readAnsibleGroupVars()
	if err != nil {
		log.Fatal(err.Error())
	}

	opts, err := generateOptions(ansibleGroupVars.Instances)
	if err != nil {
		log.Fatal(err.Error())
	}

	opts.print()
	fmt.Printf("\n\n")

	var chosenAmiNumbers []int
	var chosenOptions options
	for {
		chosenAmiNumbers, err = stringToNumbers(askUser(fmt.Sprintf("Enter ',' separated instances numbers (or %s to quit): ", colorizeRed("q"))))
		if err != nil {
			fmt.Printf(colorizeRed(err.Error() + ". Please enter valid input\n"))
			continue
		}
		chosenOptions, err = opts.filter(chosenAmiNumbers)
		if err != nil {
			fmt.Printf(colorizeRed(err.Error() + ". Please enter valid input\n"))
			continue
		}
		break
	}

	// request for prefix
	provisionHostPrefix := randStringRunes(4)

	userProvisionHostPrefix := askUser(fmt.Sprintf("Enter a prefix for the boxes (empty for random): [%s] ", colorizeYellow(provisionHostPrefix)))
	if userProvisionHostPrefix != "" {
		provisionHostPrefix = userProvisionHostPrefix
	}

	u, err := user.Current()
	if err != nil {
		log.Fatalf(err.Error())
	}
	provisionHostPrefix = fmt.Sprintf("%s-%s", u.Username, provisionHostPrefix)

	fmt.Printf("Chosen AMIs\n\n")
	for _, chosenOption := range chosenOptions {
		printVmInfo(chosenOption, provisionHostPrefix)
	}
	confirm := askUser(fmt.Sprintf("Is this right [(%s)es / (%s)o / (%s)uit]: ", colorizeGreen("y"), colorizeYellow("n"), colorizeRed("q")))

	if !(confirm == "" || confirm == "yes" || confirm == "y") {
		exit()
	}

	prepareAnsibleConfig(chosenOptions, provisionHostPrefix)

	return provisionHostPrefix
}

func cliMode() {

	var cmdCanaries = &cobra.Command{
		Use:   "canaries",
		Short: "Canary machines tools for infrastructure-agent",
		Long:  `canaries command is used for infrastructure-agent canary machines.`,
		Args:  cobra.MinimumNArgs(1),
		Run:   func(cmd *cobra.Command, args []string) {},
	}

	var cmdProvision = &cobra.Command{
		Use:   "provision",
		Short: "Provision canary machines",
		Long:  `provision is used to deploy canary machines with infrastructure-agent installed.`,
		RunE:  provisionCanaries,
	}

	// Infra agent version to install.
	cmdProvision.PersistentFlags().StringP("agent_version", "v", "", "infrastructure-agent version to deploy")
	viper.BindPFlag("agent_version", cmdProvision.PersistentFlags().Lookup("agent_version"))
	cmdProvision.MarkPersistentFlagRequired("agent_version")

	// NR license key.
	cmdProvision.PersistentFlags().StringP("license", "l", "", "infrastructure-agent license key")
	viper.BindPFlag("license", cmdProvision.PersistentFlags().Lookup("license"))
	cmdProvision.MarkPersistentFlagRequired("license")

	// Platform
	cmdProvision.PersistentFlags().StringP("platform", "p", "all", "optional platform to deploy: linux,macos,windows")
	viper.BindPFlag("platform", cmdProvision.PersistentFlags().Lookup("platform"))

	// Ansible password
	cmdProvision.PersistentFlags().StringP("ansible_password", "x", "all", "ansible password")
	viper.BindPFlag("ansible_password", cmdProvision.PersistentFlags().Lookup("ansible_password"))

	var cmdPrune = &cobra.Command{
		Use:   "prune",
		Short: "Prune canary machines",
		Long:  `prun is used to remove old canary machines.`,
		RunE:  pruneCanaries,
	}

	cmdPrune.PersistentFlags().Bool("dry_run", false, "dry run")
	viper.BindPFlag("dry_run", cmdPrune.PersistentFlags().Lookup("dry_run"))

	cmdRoot := &cobra.Command{Use: "spin-ec2"}
	cmdRoot.AddCommand(cmdCanaries)
	cmdCanaries.AddCommand(cmdProvision, cmdPrune)
	cmdRoot.Execute()
}

func canaryConfFromArgs() (canaryConf, error) {
	agentVersion := viper.GetString("agent_version")
	license := viper.GetString("license")
	platform := viper.GetString("platform")
	ansiblePassword := viper.GetString("ansible_password")

	if !semver.IsValid(agentVersion) {
		return canaryConf{}, fmt.Errorf("agent version '%s' doesn't match the pattern 'vmajor.minor.patch' format",
			agentVersion)
	}

	return canaryConf{
		license:         license,
		agentVersion:    agentVersion,
		platform:        platform,
		ansiblePassword: ansiblePassword,
	}, nil
}

// provisionCanaries will provision aws machines with the infra-agent installed.
func provisionCanaries(cmd *cobra.Command, args []string) error {

	cnf, err := canaryConfFromArgs()
	if err != nil {
		return err
	}

	if cnf.shouldProvisionLinux() {
		err := provisionLinuxCanaries(cnf)
		if err != nil {
			return err
		}
	}

	if cnf.shouldProvisionMacos() {
		err := provisionMacosCanaries(cnf)
		if err != nil {
			return err
		}
	}

	if cnf.shouldProvisionWindows() {
		err := provisionWindowsCanaries(cnf)
		if err != nil {
			return err
		}
	}

	return nil
}

func provisionMacosCanaries(cnf canaryConf) error {

	curPath, err := os.Getwd()
	if err != nil {
		return err
	}

	execNameArgs("ansible-playbook",
		"-i", path.Join(curPath, inventoryLocal),
		path.Join(curPath, "test/automated/ansible/macos-canaries.yml"))

	//rename the ansible hostname to include agent version. This is temporary until we provision macos on demand
	execNameArgs("sed", "-i.bak", fmt.Sprintf("s/canary:current/canary:%s/g", cnf.agentVersion), path.Join(curPath, inventoryMacos))
	execNameArgs("rm", fmt.Sprintf("%s.bak", path.Join(curPath, inventoryMacos)))

	var argumentsMacos = []string{
		"-e", "nr_license_key=" + cnf.license,
		"-e", "target_agent_version=" + cnf.agentVersion[1:],
		"-i", path.Join(curPath, inventoryMacos),
	}

	argumentsMacos = append(argumentsMacos, path.Join(curPath, "test/packaging/ansible/macos-canary.yml"))

	execNameArgs("ansible-playbook", argumentsMacos...)

	return nil
}

func provisionWindowsCanaries(cnf canaryConf) error {

	ansibleGroupVars, err := readAnsibleGroupVars()
	if err != nil {
		return err
	}

	opts, err := generateOptions(ansibleGroupVars.InstancesWindows())
	if err != nil {
		return err
	}

	prepareAnsibleConfig(opts, fmt.Sprintf("%s:%s", hostPrefix, cnf.agentVersion))

	return provisionEphimeralCanaries(cnf)
}

func provisionLinuxCanaries(cnf canaryConf) error {
	ansibleGroupVars, err := readAnsibleGroupVars()
	if err != nil {
		return err
	}

	opts, err := generateOptions(ansibleGroupVars.InstancesLinux())
	if err != nil {
		return err
	}

	prepareAnsibleConfig(opts, fmt.Sprintf("%s:%s", hostPrefix, cnf.agentVersion))
	//ansible password is not needed for linux
	cnf.ansiblePassword = ""

	return provisionEphimeralCanaries(cnf)
}

func provisionEphimeralCanaries(cnf canaryConf) error {
	curPath, err := os.Getwd()
	if err != nil {
		return err
	}

	execNameArgs("ansible-playbook",
		"-i", path.Join(curPath, inventoryLocal),
		"--extra-vars", "@"+path.Join(curPath, inventoryForCreation),
		"-e", "instance_prefix="+"canary:"+cnf.agentVersion+":",
		path.Join(curPath, "test/automated/ansible/provision.yml"))

	execNameArgs("ansible-playbook",
		"-i", path.Join(curPath, inventoryProvisioned),
		path.Join(curPath, "/test/automated/ansible/install-requirements.yml"))

	provisionOpts := newProvisionOptions()[OptionInstallVersionStaging]
	var playbookArguments = []string{
		"-e", "nr_license_key=" + cnf.license,
		"-e", "enable_process_metrics=true",
		"-e", "verbose=3",
		"-e", "target_agent_version=" + cnf.agentVersion[1:],
		"-i", path.Join(curPath, inventoryProvisioned),
	}
	if cnf.ansiblePassword != "" {
		playbookArguments = append(playbookArguments, "-e", "ansible_password="+cnf.ansiblePassword)
	}

	if provisionOpts.renderArgs() != "" {
		playbookArguments = append(playbookArguments, provisionOpts.renderArgs())
	}

	playbookArguments = append(playbookArguments, path.Join(curPath, provisionOpts.playbook))

	execNameArgs("ansible-playbook", playbookArguments...)

	return nil
}

// pruneCanaries removes all aws instances except the
// ones that have the latest 2 version of infra-agent installed.
func pruneCanaries(cmd *cobra.Command, args []string) error {
	dryRun := viper.GetBool("dry_run")

	instances, err := getAWSInstances(hostPrefix)
	if err != nil {
		return err
	}

	idsToTerminate, err := getInstancesToPrune(instances)
	if err != nil {
		return err
	}

	return terminateInstances(idsToTerminate, instances, dryRun)
}
