package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestServeCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantErr        bool
		expectedOutput string
	}{
		{
			name:           "serve command with help",
			args:           []string{"serve", "--help"},
			wantErr:        false,
			expectedOutput: "Start the Podcast Player API server",
		},
		{
			name:           "serve command with custom port",
			args:           []string{"serve", "--port", "9090"},
			wantErr:        false,
			expectedOutput: "",
		},
		{
			name:           "serve command with invalid port",
			args:           []string{"serve", "--port", "invalid"},
			wantErr:        true,
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			// For the actual serve command, we need to test it with a context
			// that cancels quickly to avoid actually starting the server
			if strings.Contains(tt.name, "custom port") {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				defer cancel()
				cmd.SetContext(ctx)
			}

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

func TestServeCommandFlags(t *testing.T) {
	cmd := NewRootCmd()
	serveCmd, _, err := cmd.Find([]string{"serve"})
	if err != nil {
		t.Fatalf("Failed to find serve command: %v", err)
	}

	// Test port flag
	portFlag := serveCmd.Flags().Lookup("port")
	if portFlag == nil {
		t.Error("Expected port flag to be registered")
	}

	// Test host flag
	hostFlag := serveCmd.Flags().Lookup("host")
	if hostFlag == nil {
		t.Error("Expected host flag to be registered")
	}
}
