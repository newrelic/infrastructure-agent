// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package envvar

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func ExpandInContent(content []byte) ([]byte, error) {
	content, err := removeYAMLComments(content)
	if err != nil {
		return nil, fmt.Errorf("cannot remove configuration commented lines, error: %w", err)
	}

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

// removeYAMLComments removes comments from YAML content
// golang does not support negative lookaheads
// there's an alternative library https://github.com/dlclark/regexp2 but here we stick to stdlib
// for this reason it's required:
// - several regexes
// - several capture groups that will be discarded
func removeYAMLComments(content []byte) ([]byte, error) {
	rLines := regexp.MustCompile(`(?m:^[ \t]*#.*\n)`) // ?m: = multiline flag
	matches := rLines.FindAllIndex(content, -1)

	newContent, err := removeMatches(content, matches)
	if err != nil {
		return content, err
	}

	// lines with strings: double or single quotes appearing on pairs
	rInlinedWithQuotes := regexp.MustCompile(`((.*".*".*)|(.*'.*'.*))(#.*)`)
	subMatches := rInlinedWithQuotes.FindAllSubmatchIndex(newContent, -1)

	// retrieve matches only for comment capture group
	var commentMatches [][]int
	for _, indexes := range subMatches {
		// 0,1: 1st capt group
		// ...
		// 8,9: 5th capt group <comment>
		// Ensure we only process the comment capture group
		if len(indexes) == 10 {
			// Extract the matched comment
			commentStart := indexes[8]
			commentEnd := indexes[9]
			// Check if the comment is inside quotes
			if isInsideQuotes(newContent, commentStart) {
				continue // Skip this match as it's inside quotes
			}
			// Add the comment match
			commentMatches = append(commentMatches, []int{commentStart, commentEnd})
		}
	}

	newContent, err = removeMatches(newContent, commentMatches)
	if err != nil {
		return content, err
	}

	// inlined comments within lines without quotes
	rInlinedWithoutQuotes := regexp.MustCompile(`(?m:^[^"'\n]+\s+(#.*)$)`)
	subMatches = rInlinedWithoutQuotes.FindAllSubmatchIndex(newContent, -1)

	// retrieve matches only for "comment" capture group
	commentMatches = [][]int{}
	for _, indexes := range subMatches {
		// 0,1: 1st capt group
		if len(indexes) == 4 {
			commentMatches = append(commentMatches, []int{indexes[2], indexes[3]})
		}
	}

	return removeMatches(newContent, commentMatches)
}

// Helper function to check if a position is inside quotes
func isInsideQuotes(content []byte, position int) bool {
	singleQuoteOpen := false
	doubleQuoteOpen := false

	for i := 0; i < position; i++ {
		if content[i] == '\'' && !doubleQuoteOpen {
			singleQuoteOpen = !singleQuoteOpen
		} else if content[i] == '"' && !singleQuoteOpen {
			doubleQuoteOpen = !doubleQuoteOpen
		}
	}

	// If either single or double quotes are open, the position is inside quotes
	return singleQuoteOpen || doubleQuoteOpen
}

func removeMatches(content []byte, matches [][]int) ([]byte, error) {
	if len(matches) == 0 {
		return content, nil
	}

	var newContent []byte
	var lastReplacement int
	for _, idx := range matches {
		lineStart := idx[0]
		lineEnd := idx[1]
		if len(content) < lineStart || len(content) < lineEnd {
			return content, fmt.Errorf("cannot remove commented lines from config")
		}
		newContent = append(newContent, content[lastReplacement:lineStart]...)
		lastReplacement = lineEnd

	}

	if lastReplacement != len(content) {
		newContent = append(newContent, content[lastReplacement:]...)
	}

	return newContent, nil
}
