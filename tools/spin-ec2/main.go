package main

import (
	"fmt"
	"log"
	"math/rand"
	"os/user"
	"time"
)

const (
	instancesFile = "test/automated/ansible/group_vars/localhost/main.yml"
	inventory     = "test/automated/ansible/custom-instances.yml"
	colorArm64    = "\033[32m"
	colorAmd64    = "\033[34m"
	colorReset    = "\033[0m"

	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

func main() {
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

	fmt.Printf("\nPossible provision options\n")

	provisionOpts := newProvisionOptions()
	provisionOpts.print()

	var chosenProvisionOptions provisionOptions
	for {

		chosenProvisionInput := askUser(fmt.Sprintf("Choose an option : [%s] ", colorizeYellow("0")))

		if chosenProvisionInput == ""{
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

		break
	}

	u, err := user.Current()
	if err != nil {
		log.Fatalf(err.Error())
	}
	provisionHostPrefix = fmt.Sprintf("%s-%s", u.Username, provisionHostPrefix)

	// confirm
	fmt.Printf("Chosen AMIs\n")
	for _, chosenOption := range chosenOptions {
		printVmInfo(chosenOption, provisionHostPrefix, chosenProvisionOptions)
	}
	confirm := askUser(fmt.Sprintf("Is this right [(%s)es / (%s)o / (%s)uit]: ", colorizeGreen("y"), colorizeYellow("n"), colorizeRed("q")))

	if !(confirm == "" || confirm == "yes" || confirm == "y") {
		exit()
	}

	prepareAnsibleConfig(chosenOptions, provisionHostPrefix)

	executeAnsible()
}

