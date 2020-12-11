package envvar

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func ExpandInContent(content []byte) ([]byte, error) {
	r := regexp.MustCompile(`({{ *\w+.*?}})`)
	matches := r.FindAllIndex(content, -1)

	if len(matches) == 0 {
		return content, nil
	}

	var newContent []byte
	var lastReplacement int
	for _, idx := range matches {
		evStart := idx[0] + 2 // drop {{
		evEnd := idx[1] - 2   // drop }}
		if len(content) < evStart || len(content) < evEnd {
			return content, fmt.Errorf("cannot replace configuration environment variables")
		}

		evName := strings.TrimSpace(string(content[evStart:evEnd]))
		if evVal, exist := os.LookupEnv(evName); exist {
			// quote non numerics
			if _, err := strconv.ParseFloat(evVal, 64); err != nil {
				evVal = fmt.Sprintf(`"%s"`, evVal)
			}
			newContent = append(newContent, content[lastReplacement:idx[0]]...)
			newContent = append(newContent, []byte(evVal)...)
			lastReplacement = idx[1]
		} else {
			return nil, fmt.Errorf("cannot replace configuration environment variables, missing env-var: %s", evName)
		}
	}

	if lastReplacement != len(content) {
		newContent = append(newContent, content[lastReplacement:]...)
	}

	return newContent, nil
}
