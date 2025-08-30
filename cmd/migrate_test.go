package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestMigrateCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantErr        bool
		expectedOutput string
	}{
		{
			name:           "migrate command with help",
			args:           []string{"migrate", "--help"},
			wantErr:        false,
			expectedOutput: "Manage database migrations",
		},
		{
			name:           "migrate up subcommand",
			args:           []string{"migrate", "up", "--help"},
			wantErr:        false,
			expectedOutput: "Apply all pending database migrations",
		},
		{
			name:           "migrate down subcommand",
			args:           []string{"migrate", "down", "--help"},
			wantErr:        false,
			expectedOutput: "Rollback the last applied migration",
		},
		{
			name:           "migrate status subcommand",
			args:           []string{"migrate", "status", "--help"},
			wantErr:        false,
			expectedOutput: "Display the current status",
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

			if tt.expectedOutput != "" && !strings.Contains(buf.String(), tt.expectedOutput) {
				t.Errorf("Expected output to contain %q, got %q", tt.expectedOutput, buf.String())
			}
		})
	}
}

func TestMigrateCommandSubcommands(t *testing.T) {
	cmd := NewRootCmd()
	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	if err != nil {
		t.Fatalf("Failed to find migrate command: %v", err)
	}

	// Check that subcommands exist
	expectedSubcommands := []string{"up", "down", "status"}
	for _, subCmd := range expectedSubcommands {
		found := false
		for _, child := range migrateCmd.Commands() {
			if child.Name() == subCmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected migrate command to have %q subcommand", subCmd)
		}
	}
}
