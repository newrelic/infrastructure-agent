package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
)

func askUser(message string) string {
	fmt.Printf(message)

	// get user input
	var userInput string
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		userInput = scanner.Text()
	}

	if userInput == "q" {
		exit()
	}
	return userInput
}


func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func colorizeRed(s string) string {
	return fmt.Sprintf("%s%s%s", colorRed, s, colorReset)
}

func colorizeGreen(s string) string {
	return fmt.Sprintf("%s%s%s", colorGreen, s, colorReset)
}
func colorizeYellow(s string) string {
	return fmt.Sprintf("%s%s%s", colorYellow, s, colorReset)
}

func exit() {
	fmt.Println("\nHave a nice day!")
	os.Exit(0)
}

func copyAndCapture(w io.Writer, r io.Reader) error {
	var out []byte
	buf := make([]byte, 1024, 1024)
	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			d := buf[:n]
			out = append(out, d...)
			_, err := w.Write(d)
			if err != nil {
				return err
			}
		}
		if err != nil {
			// Read returns io.EOF at the end of file, which is not an error for us
			if err == io.EOF {
				err = nil
			}
			return err
		}
	}
}

func stringToNumbers(input string) ([]int, error) {
	var opts []int
	strOpts := strings.Split(input, ",")
	for _, strOpt := range strOpts {
		opt, err := strconv.Atoi(strings.TrimSpace(strOpt))
		if err != nil {
			return nil, err
		}
		opts = append(opts, opt)
	}
	return opts, nil
}
