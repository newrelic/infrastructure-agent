package better_check

import (
	"fmt"
	"strings"

	. "gopkg.in/check.v1"
)

// -----------------------------------------------------------------------
// Has Suffix checker.

type hasSuffixChecker struct {
	*CheckerInfo
}

// The Has Suffix checker verifies that the obtained value contains the string
// of the expected value as the final component of the value. Uses strings.HasSuffix
//
// For example:
//
//     c.Assert(value, HasSuffix, "end")
//
var HasSuffix Checker = &hasSuffixChecker{
	&CheckerInfo{Name: "HasSuffix", Params: []string{"obtained", "expected"}},
}

func (checker *hasSuffixChecker) Check(params []interface{}, names []string) (result bool, error string) {
	defer func() {
		if v := recover(); v != nil {
			result = false
			error = fmt.Sprint(v)
		}
	}()
	obtained, ok := params[0].(string)
	if !ok {
		return false, "value must be a string"
	}

	expected, ok := params[1].(string)
	if !ok {
		return false, "expected must be a string"
	}

	return strings.HasSuffix(obtained, expected), ""
}
