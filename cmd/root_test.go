package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantErr        bool
		expectedOutput string
	}{
		{
			name:           "root command without args shows help",
			args:           []string{},
			wantErr:        false,
			expectedOutput: "Podcast Player API",
		},
		{
			name:           "root command with --help",
			args:           []string{"--help"},
			wantErr:        false,
			expectedOutput: "Available Commands:",
		},
		{
			name:           "root command with invalid flag",
			args:           []string{"--invalid-flag"},
			wantErr:        true,
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new root command for testing
			cmd := NewRootCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.expectedOutput != "" && !strings.Contains(buf.String(), tt.expectedOutput) {
				t.Errorf("Expected output to contain %q, got %q", tt.expectedOutput, buf.String())
			}
		})
	}
}

func TestConfigFlag(t *testing.T) {
	cmd := NewRootCmd()

	// Test that config flag is registered
	configFlag := cmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Error("Expected config flag to be registered")
		return
	}

	if configFlag.Value.String() != "" {
		t.Errorf("Expected default config value to be empty, got %s", configFlag.Value.String())
	}
}
