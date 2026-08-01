package bootstrap

import "testing"

func TestNewAccountWorkspaceAllowsAnonymousOnlyRuntime(t *testing.T) {
	workspace, err := NewAccountWorkspace(nil)
	if err != nil || workspace != nil {
		t.Fatalf("NewAccountWorkspace(nil) = %v, %v", workspace, err)
	}
}
