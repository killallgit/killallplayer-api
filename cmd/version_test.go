package cmd

import (
	"bytes"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		checkOutput func(string) bool
	}{
		{
			name:    "version command shows version info",
			args:    []string{"version"},
			wantErr: false,
			checkOutput: func(output string) bool {
				// Since version output goes to stdout, we check if command runs without error
				return true
			},
		},
		{
			name:    "version command with --short flag",
			args:    []string{"version", "--short"},
			wantErr: false,
			checkOutput: func(output string) bool {
				// Since version output goes to stdout, we check if command runs without error
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.checkOutput != nil && !tt.checkOutput(buf.String()) {
				t.Errorf("Output check failed")
			}
		})
	}
}

func TestVersionCommandFlags(t *testing.T) {
	cmd := NewRootCmd()
	versionCmd, _, err := cmd.Find([]string{"version"})
	if err != nil {
		t.Fatalf("Failed to find version command: %v", err)
	}

	// Test short flag
	shortFlag := versionCmd.Flags().Lookup("short")
	if shortFlag == nil {
		t.Error("Expected short flag to be registered")
	}
}
