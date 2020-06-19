package better_check

import (
	"fmt"
	"strings"

	. "gopkg.in/check.v1"
)

// -----------------------------------------------------------------------
// Has Prefix checker.

type hasPrefixChecker struct {
	*CheckerInfo
}

// The Has Prefix checker verifies that the obtained value contains the string
// of the expected value as the initial component of the value. Uses strings.HasPrefix
//
// For example:
//
//     c.Assert(value, HasPrefix, "end")
//
var HasPrefix Checker = &hasPrefixChecker{
	&CheckerInfo{Name: "HasPrefix", Params: []string{"obtained", "expected"}},
}

func (checker *hasPrefixChecker) Check(params []interface{}, names []string) (result bool, error string) {
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

	return strings.HasPrefix(obtained, expected), ""
}
