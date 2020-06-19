package better_check

import (
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type BetterCheckSuite struct {
}

var _ = Suite(&BetterCheckSuite{})

func (s *BetterCheckSuite) TestApproxEquals(c *C) {
	value := 42.1

	c.Assert(value, ApproxEquals, 42.0, 0.5)
}

func (s *BetterCheckSuite) TestHasSuffix(c *C) {
	value := "beginMiddleEnd"

	c.Assert(value, HasSuffix, "End")
}

func (s *BetterCheckSuite) TestHasPrefix(c *C) {
	value := "beginMiddleEnd"

	c.Assert(value, HasPrefix, "begin")
}

func (s *BetterCheckSuite) TestContains(c *C) {
	value := "beginMiddleEnd"

	c.Assert(value, Contains, "Middle")
}
