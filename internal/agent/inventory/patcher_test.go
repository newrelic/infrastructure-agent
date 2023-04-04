package inventory

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestPatcher_NeedsCleanup_NeverCleaned(t *testing.T) {
	b := BasePatcher{}
	assert.False(t, b.needsCleanup())
	assert.False(t, b.needsCleanup())
}

func TestPatcher_NeedsCleanup_DefaultRemoveEntitiesPeriodExceeded(t *testing.T) {
	b := BasePatcher{
		lastClean: time.Now().Add(-defaultRemoveEntitiesPeriod),
	}
	assert.True(t, b.needsCleanup())
	assert.False(t, b.needsCleanup())
}

func TestPatcher_NeedsCleanup_ConfigRemoveEntitiesPeriodExceeded(t *testing.T) {
	b := BasePatcher{
		cfg: PatcherConfig{
			RemoveEntitiesPeriod: 1 * time.Hour,
		},
		lastClean: time.Now().Add(-30 * time.Minute),
	}
	assert.False(t, b.needsCleanup())

	b.lastClean = b.lastClean.Add(-31 * time.Minute)
	assert.True(t, b.needsCleanup())
	assert.False(t, b.needsCleanup())
}

func TestPatcher_Ignored(t *testing.T) {
	b := BasePatcher{
		cfg: PatcherConfig{
			IgnoredPaths: map[string]struct{}{"test": {}},
		},
	}
	assert.True(t, b.isIgnored("test"))
	assert.False(t, b.isIgnored("test2"))
}
