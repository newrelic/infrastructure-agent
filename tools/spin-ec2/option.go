package main

import (
	"fmt"
	"strings"
)

type architecture string

func (a architecture) color() string {
	if a == "amd64" {
		return colorAmd64
	}

	return colorArm64
}

type option struct {
	id       int
	arch     architecture
	os       string
	instance instanceDef
}

func (o option) FullName() string {
	optionFormat := "arch:%s%6s%s os:%18s"
	return fmt.Sprintf(optionFormat, o.arch.color(), o.arch, colorReset, o.os)
}
func (o option) Option() string {
	optionFormat := "[%2d] %s%6s%s %18s"
	return fmt.Sprintf(optionFormat, o.id, o.arch.color(), o.arch, colorReset, o.os)
}

type options map[int]option

func (o options) print() {
	for i := 0; i < len(o)/2+1; i++ {
		fmt.Print(o[i].Option())
		if _, ok := o[i+len(o)/2]; ok {
			fmt.Printf("        %s\n", o[i+len(o)/2].Option())
		}
	}
}

func (o options) filter(optionsIds []int) (options, error) {
	filtered := make(options)
	for _, optId := range optionsIds {
		if opt, ok := o[optId]; ok {
			filtered[optId] = opt
		} else {
			return nil, fmt.Errorf("%v is not a valid option", optId)
		}
	}
	return filtered, nil
}

func generateOptions(instances []instanceDef) (options, error) {

	opts := options{}

	for i, instance := range instances {

		arch := instance.Name[:strings.Index(instance.Name, ":")]
		opSystem := instance.Name[strings.Index(instance.Name, ":")+1:]

		opts[i] = option{
			id:       i,
			arch:     architecture(arch),
			os:       opSystem,
			instance: instance,
		}
	}

	return opts, nil
}

func printVmInfo(chosenOption option, provisionHostPrefix string) {
	fmt.Printf("Os: %s%s%s\n", colorPurple, chosenOption.os, colorReset)
	fmt.Printf("Arch: %s%s%s\n", chosenOption.arch.color(), chosenOption.arch, colorReset)
	fmt.Printf("Prefix: %s%s%s\n", colorCyan, provisionHostPrefix, colorReset)
	fmt.Printf("\n\n")
}
