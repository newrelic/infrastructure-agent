package better_check

import (
	"fmt"

	. "gopkg.in/check.v1"
)

// -----------------------------------------------------------------------
// Approximately Equals checker.

type approxEqualsChecker struct {
	*CheckerInfo
}

// The Approximately Equals checker verifies that the obtained value is equal to
// the expected value witin a range, according to usual Go semantics for >= and <=
// for floating numbers
//
// For example:
//
//     c.Assert(value, ApproxEquals, 42.0, 0.5)
//
var ApproxEquals Checker = &approxEqualsChecker{
	&CheckerInfo{Name: "ApproxEquals", Params: []string{"obtained", "expected", "tolerance"}},
}

func (checker *approxEqualsChecker) Check(params []interface{}, names []string) (result bool, error string) {
	defer func() {
		if v := recover(); v != nil {
			result = false
			error = fmt.Sprint(v)
		}
	}()
	obtained, ok := params[0].(float64)
	if !ok {
		return false, "value must be a float64"
	}

	expected, ok := params[1].(float64)
	if !ok {
		return false, "expected must be a float64"
	}

	tolerance, ok := params[2].(float64)
	if !ok {
		return false, "tolerance must be a float64"
	}

	return (obtained >= (expected - tolerance)) && (obtained <= (expected + tolerance)), ""
}
