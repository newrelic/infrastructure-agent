// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/helpers/lru"
)

var (
	JsonFilesRegexp       = regexp.MustCompile("^[^~]+.json$")
	sanitizeFileNameCache = lru.New()
	HiddenField           = "<HIDDEN>"
)

var quotations = map[uint8]bool{
	'"':  true,
	'\'': true,
	'`':  true,
}

const (
	MaxBackoffErrorCount = 31
	// The Maximum size of the cache used to store sanitized file names.
	SanitizeFileNameCacheSize = 1000
)

// Calculates the backoff as a function of the base and the maximum intervals, and the count of retries
func ExpBackoff(base, max time.Duration, count uint32) time.Duration {
	// can only shift to the 31st bit return max
	if count >= MaxBackoffErrorCount {
		return max
	}
	// bitshift to get pow2 cheaply
	backoff := time.Duration(1<<(count-1))*time.Second + base
	if backoff > max {
		return max
	}
	return backoff
}

// Helper to compute the MD5 digest of a particular string
func GenerateMD5(data string) ([]byte, error) {
	var digestBuffer bytes.Buffer

	md5Buffer := md5.New()
	_, _ = io.WriteString(md5Buffer, data)
	digestBuffer.Write(md5Buffer.Sum(nil))

	return digestBuffer.Bytes(), nil
}

// presupposes a regular file
func FileMD5(filename string) (hash []byte, err error) {
	var f *os.File
	if f, err = os.Open(filename); err != nil {
		return
	}
	defer CloseQuietly(f)

	h := md5.New()
	if _, err = io.Copy(h, f); err != nil {
		return
	}
	hash = h.Sum(nil)

	return
}

func FlattenJson(parentKey string, data map[string]interface{}, jsonMap map[string]interface{}) map[string]interface{} {
	var flatKey, flatValue string
	for k, v := range data {
		flatKey = fmt.Sprintf("%s/%s", parentKey, k)
		flatKey = strings.TrimPrefix(flatKey, "/")
		strValue, ok := v.(string)
		if ok {
			jsonMap[flatKey] = strValue
		} else if reflect.TypeOf(v) == reflect.TypeOf(data) {
			jsonMap = FlattenJson(flatKey, v.(map[string]interface{}), jsonMap)
		} else {
			switch v.(type) {
			case bool:
				flatValue = fmt.Sprintf("%v", v)
			case float64:
				flatValue = fmt.Sprintf("%v", v)
			case nil:
				flatValue = "null"
			case []interface{}:
				b, err := json.Marshal(v)
				if err != nil {
					flatValue = fmt.Sprintf("could not serialize json: %s", err.Error())
				}
				flatValue = fmt.Sprintf("%s", b)
			default:
				flatValue = fmt.Sprintf("%s", v)
			}
			jsonMap[flatKey] = flatValue
		}
	}
	return jsonMap
}

var ISO8601RE = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})T(\d{2})\:(\d{2})\:(\d{2})([zZ]|[\+|-]\d{2}:\d{2})`)

var SensitiveKeys = []string{"key", "secret", "password", "token", "passphrase", "credential"}

func SanitizeJson(jsonMap map[string]interface{}) map[string]interface{} {
	for k, v := range jsonMap {
		for _, str := range SensitiveKeys {
			if strings.Contains(strings.ToLower(k), str) {
				md5sum := md5.Sum([]byte(fmt.Sprintf("%s", v)))
				jsonMap[k] = fmt.Sprintf("Secret obfuscated - md5 hash: %x", md5sum)
			}
		}
	}

	return jsonMap
}

// Cleans the command line to remove wrappers like quotation marks.
func SanitizeCommandLine(cmd string) string {
	cleanCommand := strings.TrimSpace(cmd)
	if len(cleanCommand) > 0 && quotations[cleanCommand[0]] {
		cleanCommand = strings.Replace(cleanCommand, cleanCommand[:1], "", 2)
	}
	return cleanCommand
}

// GetEnv retrieves the environment variable key. If it does not exist it returns the default.
// From gopsutil
func GetEnv(key string, dfault string, combineWith ...string) string {
	value := os.Getenv(key)
	if value == "" {
		value = dfault
	}

	switch len(combineWith) {
	case 0:
		return value
	case 1:
		return filepath.Join(value, combineWith[0])
	default:
		all := make([]string, len(combineWith)+1)
		all[0] = value
		copy(all[1:], combineWith)
		return filepath.Join(all...)
	}
}

// LogStructureDetails transforms a Go structure into json and logs it as a payload trace
func LogStructureDetails(logEntry log.Entry, sample interface{}, name, phase string, optionalFields logrus.Fields) {
	// prevent json marshall if debug is not enabled
	if logEntry.IsDebugEnabled() {
		if name != "" {
			logEntry = logEntry.WithField("structure", name)
		}
		if phase != "" {
			logEntry = logEntry.WithField("location", phase)
		}
		buffer, dErr := json.Marshal(sample)
		if dErr != nil {
			logEntry.WithError(dErr).Debug("Can't marshal sample.")
		} else {
			logEntry.WithTraceField("payload", string(buffer)).WithFields(optionalFields).Debug("Received sampler payload")
		}
	}
}

// DebugStackf logs a formatted debug message with information of the invokers' stacktrace.
// This function makes sure that no memory is generated if logger level lower to log.DebugLevel. This function is
// aimed only to debug messages from functions that can be invoked from multiple parts of the code.
func DebugStackf(format string, args ...interface{}) {
	if log.IsLevelEnabled(logrus.DebugLevel) {
		const stackTraceLength = 2

		callers := make([]string, 0, stackTraceLength)
		// Getting callers information
		fpcs := make([]uintptr, stackTraceLength)
		n := runtime.Callers(2, fpcs)
		for i := n - 1; i >= 0; i-- {
			// get the info of the actual function that's in the pointer
			caller := runtime.FuncForPC(fpcs[i] - 1)
			if caller != nil {
				callers = append(callers, caller.Name())
			}
		}
		log.WithField("stacktrace", strings.Join(callers, " -> ")).Debug(fmt.Sprintf(format, args...))
	}
}

// Removes the next characters from a string so the resulting string can be used as a file name in any major operating
// system (Unix/Linux, Windows and Mac).
// - slash (/): folder separator in Unix
// - backslash (\): folder separator in Windows
// - colon (:): mountpoint drive in Windows
// - asterisk (*): wilcard in Unix and Windows
// - question mark (?): wildcard in Unix and Windows
// - quote ("): beginning and end of filename containing spaces in Windows
// - less than (<): redirect input
// - greater than (>): redirect output
// - vertical bar (|): software pipelining in Unix and Windows
func SanitizeFileName(fileName string) string {
	if fileName == "" {
		return fileName
	}
	value, found := sanitizeFileNameCache.Get(fileName)
	if found {
		if stringValue, isString := value.(string); isString {
			return stringValue
		}
	}
	// SanitizeFileName versions for Windows and Posix
	sanitized := make([]rune, len(fileName))
	i := 0
	for _, c := range fileName {
		switch c {
		case '\\':
		case '/':
		case ':':
		case '*':
		case '?':
		case '"':
		case '<':
		case '>':
		case '|':
		default:
			sanitized[i] = c
			i++
		}
	}
	result := string(sanitized[:i])

	// Store it to cache and prevent cache growing too big.
	sanitizeFileNameCache.RemoveUntilLen(SanitizeFileNameCacheSize)
	sanitizeFileNameCache.Add(fileName, result)

	return result
}

// RemoveEmptyAndDuplicateEntries removes the empty and duplicate entries of a string slice, keeping the original
// order of the input slice
func RemoveEmptyAndDuplicateEntries(entries []string) []string {
	uniques := make([]string, 0, len(entries))
	uniquesMap := make(map[string]interface{})

	for _, entry := range entries {
		if entry != "" && uniquesMap[entry] == nil {
			uniques = append(uniques, entry)
			uniquesMap[entry] = 1
		}
	}

	return uniques
}

// ReadFirstLine will return just the first line of the file.
func ReadFirstLine(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		log.WithError(err).WithField("path", filename).Error("can't open file")
		return "unknown"
	}
	defer CloseQuietly(file)
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	output := scanner.Text()
	if err := scanner.Err(); err != nil {
		log.WithError(err).WithField("path", filename).Error("can't read file")
		return "unknown"
	}
	return strings.TrimSpace(output)
}

func ObfuscateSensitiveDataFromError(originalError error) (obfuscateError error) {
	if originalError == nil {
		return originalError
	}

	matched, _, result := ObfuscateSensitiveData(originalError.Error())
	if matched {
		return errors.New(result)
	}
	return originalError
}

func ObfuscateSensitiveDataFromString(originalString string) (obfuscateString string) {
	if originalString == "" {
		return originalString
	}

	matched, _, result := ObfuscateSensitiveData(originalString)
	if matched {
		return result
	}
	return originalString
}

// ObfuscateSensitiveDataFromMap will is used to detect sensitive data like tokens/passwords etc and
// replace them by *. If a key is matched is considered a field(matched but no tokens/passwords founded)
// the value will be obfuscated.
func ObfuscateSensitiveDataFromMap(data map[string]string) map[string]string {
	result := make(map[string]string, len(data))

	for key, value := range data {
		matched, isField, resultKey := ObfuscateSensitiveData(key)

		if matched && isField {
			value = HiddenField
		}
		result[resultKey] = value
	}
	return result
}

// ObfuscateSensitiveDataFromArray will is used to detect sensitive data like tokens/passwords etc and
// replace them by *. If a value is matched and is considered a field(matched but no tokens/passwords founded)
// the next value will be considered related so obfuscated.
func ObfuscateSensitiveDataFromArray(data []string) []string {
	var result []string

	var obfuscateNext bool
	for _, value := range data {

		if obfuscateNext {
			result = append(result, HiddenField)
			obfuscateNext = false
			continue
		}
		matched, isField, resultValue := ObfuscateSensitiveData(value)

		if matched && isField {
			obfuscateNext = true
		}

		result = append(result, resultValue)
	}

	return result
}

//nolint:gochecknoglobals
var obfuscateRegexes = []*regexp.Regexp{
	// Match if contains pass|token|cert|auth|key|secret|salt|cred|pw
	// and capturing if found the group after one of the separators: ' ', ':', '=' and '"'.
	regexp.MustCompile(`(?i)(?:pass|token|cert|auth|key|secret|salt|cred|pw)(?:[^\s:="]*)(?:[\s:="]*)([^\s:="]+)?`),
	// Match password in url http://user:pass@localhost
	regexp.MustCompile(`(?i)(?:\:\/\/\w+)(?:[\s:="]*)([a-zA-Z0-9]+)(?:[\@])`),
}

// ObfuscateSensitiveData is used to detect sensitive data like tokens/passwords etc and
// replace them by *.
// e.g. NRIA_CUSTOM_PASSWORD=1234               => NRIA_CUSTOM_PASSWORD=*
// It will also obfuscate passwords defined in urls with the format: http://user:pass@localhost
//
//	/usr/bin/custom_cmd -pwd 1234 -arg2 abc => /usr/bin/custom_cmd -pwd * -arg2 abc
func ObfuscateSensitiveData(value string) (matched, isField bool, result string) {
	result = value

	for _, obfuscateRegex := range obfuscateRegexes {

		matches := obfuscateRegex.FindAllStringSubmatchIndex(result, -1)

		var transforms bytes.Buffer

		lastEndIndex := 0

		for _, indexes := range matches {
			// Expect array of 4:
			// start-end indexes of the full match
			// start-end indexes of the group 1 (data that should be obfuscated)
			if len(indexes) != 4 {
				break
			}

			startIndex := indexes[2]
			endIndex := indexes[3]

			// If the group 1 was not present there is nothing to obfuscate.
			if startIndex == -1 || endIndex == -1 {
				isField = len(matches) == 1

				break
			}

			transforms.WriteString(result[lastEndIndex:startIndex])
			transforms.WriteString(HiddenField)
			lastEndIndex = endIndex
		}

		if len(matches) > 0 {
			matched = true

			transforms.WriteString(result[lastEndIndex:])
			result = transforms.String()
		}
	}

	return
}

func CloseQuietly(file io.Closer) {
	_ = file.Close()
}

// SplitRightSubstring returns the first remaining part from the end of a substring
// and a given separator, if no substring or separator is found, and empty
// string is returned.
// Example: SplitRightSubstring("Hello: bye$", "Hello: ", "$") -> "bye"
func SplitRightSubstring(output, substring, separator string) string {
	idx := strings.Index(output, substring)
	// substring not found
	if idx == -1 {
		return ""
	}
	start := idx + len(substring)
	right := strings.Index(output[start:], separator)
	// separator not found
	if right == -1 {
		return ""
	}

	return output[start : start+right]
}
