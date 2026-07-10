// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package delta

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSavePluginSource_RejectsPathTraversal is a regression test for NR-574888:
// an integration payload's `name` becomes the inventory plugin `term`, which was
// concatenated directly into the on-disk path. A `../` in `term` let an
// (unauthenticated, root) writer escape the data directory. The write must now be
// contained to the plugin directory.
func TestSavePluginSource_RejectsPathTraversal(t *testing.T) {
	dataDir := t.TempDir()
	s := NewStore(dataDir, "default", 1024*1024, true)

	malicious := []string{
		"../../../../../../../../etc/nr_pwned",
		"..",
		"../nr_pwned",
		"foo/bar",
		`..\..\windows\system32\evil`,
		"a/../../b",
	}
	for _, term := range malicious {
		if err := s.SavePluginSource("localentity", "integration", term, map[string]interface{}{"marker": "x"}); err == nil {
			t.Errorf("expected SavePluginSource to reject malicious term %q, got nil error", term)
		}
	}

	// Nothing must have escaped the data directory.
	if _, err := os.Stat("/etc/nr_pwned.json"); err == nil {
		t.Fatalf("path traversal wrote /etc/nr_pwned.json")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dataDir), "nr_pwned.json")); err == nil {
		t.Fatalf("path traversal escaped the data dir")
	}

	// A legitimate integration name (dots allowed) still writes, inside the plugin dir.
	const goodTerm = "com.newrelic.mysql"
	if err := s.SavePluginSource("localentity", "integration", goodTerm, map[string]interface{}{"k": "v"}); err != nil {
		t.Fatalf("valid term %q was rejected: %v", goodTerm, err)
	}
	want := filepath.Join(s.PluginDirPath("integration", "localentity"), goodTerm+".json")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("valid inventory not written at expected path %s: %v", want, err)
	}
}
