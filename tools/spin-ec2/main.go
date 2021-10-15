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
	inventoryLinux       = "test/automated/ansible/inventory.ec2"
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

	if !skipVMCreation {
		createVMs()
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

	curPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	if !skipVMCreation {
		execNameArgs("ansible-playbook",
			"-i", path.Join(curPath, inventoryLocal),
			"--extra-vars", "@"+path.Join(curPath, inventoryForCreation),
			path.Join(curPath, "test/automated/ansible/provision.yml"))

		execNameArgs("ansible-playbook",
			"-i", path.Join(curPath, inventoryLinux),
			path.Join(curPath, "test/automated/ansible/install-requirements.yml"))
	}

	if len(chosenProvisionOptions) > 0 {

		for _, chosenOpt := range chosenProvisionOptions {

			if chosenOpt.playbook == "" {
				continue
			}

			var arguments []string

			arguments = append(arguments, "-i", path.Join(curPath, inventoryLinux))

			if chosenOpt.renderArgs() != "" {
				arguments = append(arguments, chosenOpt.renderArgs())
			}

			arguments = append(arguments, "-e", "nr_license_key="+license)

			arguments = append(arguments, path.Join(curPath, chosenOpt.playbook))

			execNameArgs("ansible-playbook", arguments...)
		}
	}
}

func createVMs() {
	rand.Seed(time.Now().UnixNano())
	var err error

	ansibleGroupVars, err := readAnsibleGroupVars()
	if err != nil {
		log.Fatal(err.Error())
	}

	opts, err := generateOptions(*ansibleGroupVars)
	if err != nil {
		log.Fatal(err.Error())
	}

	opts.print()

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

// provisionCanaries will provision aws machines with the infra-agent installed.
func provisionCanaries(cmd *cobra.Command, args []string) error {
	agentVersion := viper.GetString("agent_version")
	license := viper.GetString("license")

	if !semver.IsValid(agentVersion) {
		return fmt.Errorf("agent version '%s' doesn't match the pattern 'vmajor.minor.patch' format",
			agentVersion)
	}

	err := provisionLinuxCanaries(license, agentVersion)

	if err != nil {
		return err
	}

	return provisionMacosCanaries(license, agentVersion)
}

func provisionMacosCanaries(license string, agentVersion string) error {

	curPath, err := os.Getwd()
	if err != nil {
		return err
	}

	execNameArgs("ansible-playbook",
		"-i", path.Join(curPath, inventoryLocal),
		path.Join(curPath, "test/automated/ansible/macos-canaries.yml"))

	var argumentsMacos = []string{
		"-e", "nr_license_key=" + license,
		"-e", "target_agent_version=" + agentVersion[1:],
		"-i", path.Join(curPath, inventoryMacos),
	}

	argumentsMacos = append(argumentsMacos, path.Join(curPath, "test/packaging/ansible/macos-canary.yml"))

	execNameArgs("ansible-playbook", argumentsMacos...)

	return nil
}

func provisionLinuxCanaries(license, agentVersion string) error {
	ansibleGroupVars, err := readAnsibleGroupVars()
	if err != nil {
		return err
	}

	opts, err := generateOptions(*ansibleGroupVars)
	if err != nil {
		return err
	}

	prepareAnsibleConfig(opts, fmt.Sprintf("%s:%s", hostPrefix, agentVersion))

	curPath, err := os.Getwd()
	if err != nil {
		return err
	}

	execNameArgs("ansible-playbook",
		"-i", path.Join(curPath, inventoryLocal),
		"--extra-vars", "@"+path.Join(curPath, inventoryForCreation),
		"-e", "instance_prefix="+"canary:"+agentVersion+":",
		path.Join(curPath, "test/automated/ansible/provision.yml"))

	execNameArgs("ansible-playbook",
		"-i", path.Join(curPath, inventoryLinux),
		path.Join(curPath, "/test/automated/ansible/install-requirements.yml"))

	provisionOpts := newProvisionOptions()[OptionInstallVersionStaging]
	var argumentsLinux = []string{
		"-e", "nr_license_key=" + license,
		"-e", "enable_process_metrics=true",
		"-e", "verbose=3",
		"-e", "target_agent_version=" + agentVersion[1:],
		"-i", path.Join(curPath, inventoryLinux),
	}

	if provisionOpts.renderArgs() != "" {
		argumentsLinux = append(argumentsLinux, provisionOpts.renderArgs())
	}

	argumentsLinux = append(argumentsLinux, path.Join(curPath, provisionOpts.playbook))

	execNameArgs("ansible-playbook", argumentsLinux...)

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
