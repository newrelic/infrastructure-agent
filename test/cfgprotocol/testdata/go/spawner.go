// +build integration
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"strings"
	"time"
)

func main() {
	path := flag.String("path", "", "")
	multiLine := flag.Bool("multiLine", false, "")
	times := flag.Int("times", 100, "")
	sleepTime := flag.Duration("sleepTime", 2*time.Second, "")
	mode := flag.String("mode", "short", "")
	flag.String("nri-process-name", "unknown", "")
	flag.Parse()
	content, err := ioutil.ReadFile(*path)
	if err != nil {
		panic(err)
	}
	contentStr := string(content)
	if !*multiLine {
		contentStr = strings.ReplaceAll(contentStr, "\n", "")
	}
	switch strings.ToLower(*mode) {
	case "long":
		for i := 0; i < *times; i++ {
			fmt.Println(contentStr)
			time.Sleep(*sleepTime)
		}
	case "short":
		fmt.Println(contentStr)
		time.Sleep(*sleepTime)
	default:
		panic("unsupported running mode!")
	}

}
