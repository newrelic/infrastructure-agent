package better_check

import (
	"fmt"
	"strings"

	. "gopkg.in/check.v1"
)

// -----------------------------------------------------------------------
// Contains checker.

type containsChecker struct {
	*CheckerInfo
}

// The Contains checker verifies that the obtained value contains the string
// of the expected value. Uses strings.Contains
//
// For example:
//
//     c.Assert(value, Contains, "end")
//
var Contains Checker = &containsChecker{
	&CheckerInfo{Name: "Contains", Params: []string{"obtained", "expected"}},
}

func (checker *containsChecker) Check(params []interface{}, names []string) (result bool, error string) {
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

	return strings.Contains(obtained, expected), ""
}
