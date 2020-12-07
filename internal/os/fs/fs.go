package fs

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func ReadFirstLine(filename string) (line string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		err = fmt.Errorf("cannot open file %s, err: %s", filename, err)
		line = "unknown"
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()
	output := scanner.Text()
	if err = scanner.Err(); err != nil {
		err = fmt.Errorf("cannot read file %s, err: %s", filename, err)
		line = "unknown"
		return
	}

	line = strings.TrimSpace(output)
	return
}

func ReadFileFieldMatching(filename string, re *regexp.Regexp) (value string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		err = fmt.Errorf("cannot open file %s, err: %s", filename, err)
		value = "unknown"
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := re.FindStringSubmatch(scanner.Text()); len(line) > 1 {
			value = line[1]
			return
		}
	}

	if err := scanner.Err(); err != nil {
		err = fmt.Errorf("cannot read file %s, err: %s", filename, err)
	}
	value = "unknown"
	return
}
