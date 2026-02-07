package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Helper function to create a temporary config file for tests
func createTempConfigFile(t *testing.T, content string) string {
	tmpFile, err := ioutil.TempFile("", "repomon-config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write to temp config file: %v", err)
	}
	return tmpFile.Name()
}

// Helper to initialize a git repo and make a commit
func setupDummyGitRepo(t *testing.T, path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("Failed to create dummy repo dir: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = path
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git init in %s: %v\nOutput: %s", path, err, string(output))
	}

	// Create a dummy file
	dummyFilePath := filepath.Join(path, "README.md")
	if err := ioutil.WriteFile(dummyFilePath, []byte("Hello, Git!"), 0644); err != nil {
		t.Fatalf("Failed to write dummy file: %v", err)
	}

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = path
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git add: %v\nOutput: %s", err, string(output))
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = path
	// Set dummy committer info for CI environments
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test User", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test User", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to git commit: %v\nOutput: %s", err, string(output))
	}
}


func TestExecuteList(t *testing.T) {
	// Create testdata directory and a dummy repo1
	testdataDir := filepath.Join(os.TempDir(), "repomon-testdata")
	dummyRepoPath := filepath.Join(testdataDir, "repo1")
	if err := os.MkdirAll(dummyRepoPath, 0755); err != nil {
		t.Fatalf("Failed to create testdata directory: %v", err)
	}
	defer os.RemoveAll(testdataDir)


	tests := []struct {
		name           string
		configContent  string
		rootOpts       *rootOptions
		expectedOutput string
		expectedError  string
	}{
		{
			name: "List default group with local and remote repos",
			configContent: `
[defaults]
days = 1

[groups.default]
repos = [
    "__TESTDATA_DIR__",
    "https://github.com/test/remote-repo",
]
`,
			rootOpts: &rootOptions{
				group: "", // Should default to "default"
			},
			expectedOutput: fmt.Sprintf("Repositories for group 'default':\n  - repo1: %s\n  - remote-repo: https://github.com/test/remote-repo (remote)\n", dummyRepoPath),
			expectedError:  "",
		},
		{
			name: "List specific group with no repos",
			configContent: `
[groups.emptygroup]
repos = []
`,
			rootOpts: &rootOptions{
				group: "emptygroup",
			},
			expectedOutput: "No repositories found for group 'emptygroup'.\n",
			expectedError:  "",
		},
		{
			name: "List non-existent group, fallback to default",
			configContent: `
[groups.default]
repos = ["__TESTDATA_DIR__"]
`,
			rootOpts: &rootOptions{
				group: "nonexistent",
			},
			expectedOutput: fmt.Sprintf("Repositories for group 'default':\n  - repo1: %s\n", dummyRepoPath), // Now uses effectiveGroupName
			expectedError:  "",
		},
		{
			name: "Config file not found",
			configContent: `
`, // Empty content for a non-existent file
			rootOpts: &rootOptions{
				configFile: "/path/does/not/exist/config.toml",
				group:      "default",
			},
			expectedOutput: "",
			expectedError:  "failed to load configuration: config file not found: /path/does/not/exist/config.toml",
		},
		{
			name: "Config file is empty",
			configContent: ``, // Empty config file content
			rootOpts: &rootOptions{
				group: "default",
			},
			expectedOutput: "", // Now empty because config.Load returns error, and executeList exits early
			expectedError:  "failed to get repositories: no default group found in configuration",
		},
		{
			name: "Config file with no default group and non-existent group requested",
			configContent: `
[groups.other]
repos = ["__TESTDATA_DIR__"]
`,
			rootOpts: &rootOptions{
				group: "nonexistent",
			},
			expectedOutput: "", // No output because GetRepos will return an error due to no default group fallback
			expectedError:  "failed to get repositories: no default group found in configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputBuf := new(bytes.Buffer)
			errorBuf := new(bytes.Buffer)

			var cfgPath string
			// Handle "Config file not found" specifically
			if strings.Contains(tt.expectedError, "config file not found") {
				cfgPath = tt.rootOpts.configFile // Keep the non-existent path
			} else {
				// For other cases, create a temporary config file
				formattedConfigContent := strings.Replace(tt.configContent, "__TESTDATA_DIR__", dummyRepoPath, -1) // Use dummyRepoPath here for the replacement
				cfgPath = createTempConfigFile(t, formattedConfigContent)
				defer os.Remove(cfgPath)
			}
			tt.rootOpts.configFile = cfgPath // Ensure rootOpts has the correct config file path for the test

			err := executeList(nil, tt.rootOpts, outputBuf, errorBuf) // args not used by executeList

			if tt.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Test %s:\nExpected error containing '%s'\nGot: %v", tt.name, tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("Test %s:\nUnexpected error: %v", tt.name, err)
			}

			if outputBuf.String() != tt.expectedOutput {
				t.Errorf("Test %s:\nExpected output:\n%q\nGot:\n%q", tt.name, tt.expectedOutput, outputBuf.String())
			}

			// We are not explicitly checking errorBuf for now, but it's available for debug/future.
			if errorBuf.Len() > 0 {
				t.Logf("Test %s:\nError output (slog):\n%s", tt.name, errorBuf.String())
			}
		})
	}
}


func TestExecuteRun(t *testing.T) {
	// Create testdata directory for dummy repos
	testdataDir := filepath.Join(os.TempDir(), "repomon-testdata-run")
	dummyRepoPath := filepath.Join(testdataDir, "dummy-repo")
	defer os.RemoveAll(testdataDir)

	setupDummyGitRepo(t, dummyRepoPath) // Setup the dummy git repo

	tests := []struct {
		name           string
		configContent  string
		runOpts        *runOptions
		rootOpts       *rootOptions
		expectedOutput string
		expectedError  string
		expectedLog    string
	}{
		{
			name: "Successful run with local repo",
			configContent: fmt.Sprintf(`
[defaults]
days = 1

[groups.default]
repos = ["%s"]
`, dummyRepoPath),
			runOpts: &runOptions{
				days:  1,
				debug: false,
			},
			rootOpts: &rootOptions{
				group: "default",
			},
			expectedOutput: "Repository Monitor Report",
			expectedError:  "",
			expectedLog:    "",
		},
		{
			name: "Config file not found",
			configContent: ``,
			runOpts: &runOptions{
				days: 1,
			},
			rootOpts: &rootOptions{
				configFile: "/path/does/not/exist/config.toml",
				group:      "default",
			},
			expectedOutput: "",
			expectedError:  "failed to load configuration: config file not found: /path/does/not/exist/config.toml",
			expectedLog:    "Failed to load configuration",
		},
		{
			name: "No default group found in configuration (error)",
			configContent: `
[groups.other]
repos = ["__TESTDATA_DIR__"]
`, // Use placeholder for path
			runOpts: &runOptions{
				days: 1,
			},
			rootOpts: &rootOptions{
				group: "nonexistent",
			},
			expectedOutput: "",
			expectedError:  "failed to get repositories: no default group found in configuration",
			expectedLog:    "Failed to get repositories",
		},
		{
			name: "Empty config file (error)",
			configContent: ``,
			runOpts: &runOptions{
				days: 1,
			},
			rootOpts: &rootOptions{
				group: "default",
			},
			expectedOutput: "",
			expectedError:  "failed to get repositories: no default group found in configuration", // Error from GetRepos if Load succeeds with empty config
			expectedLog:    "Failed to get repositories",
		},
		{
			name: "No repositories found for group (output message)",
			configContent: `
[groups.emptygroup]
repos = []
`,
			runOpts: &runOptions{
				days: 1,
			},
			rootOpts: &rootOptions{
				group: "emptygroup",
			},
			expectedOutput: "No recent commits found in any repository.",
			expectedError:  "",
			expectedLog:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputBuf := new(bytes.Buffer)
			errorBuf := new(bytes.Buffer)

			var cfgPath string
			// Handle "Config file not found" specifically
			if strings.Contains(tt.expectedError, "config file not found") {
				cfgPath = tt.rootOpts.configFile // Keep the non-existent path
			} else {
				// For other cases, create a temporary config file
				formattedConfigContent := strings.Replace(tt.configContent, "__TESTDATA_DIR__", dummyRepoPath, -1)
				cfgPath = createTempConfigFile(t, formattedConfigContent)
				defer os.Remove(cfgPath)
			}
			tt.rootOpts.configFile = cfgPath // Ensure rootOpts has the correct config file path for the test


			// Cobra command context for executeRun. Use context.Background() for tests.
			ctx := context.Background()


			err := executeRun(ctx, nil, tt.runOpts, tt.rootOpts, outputBuf, errorBuf) // args not used by executeRun


			if tt.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Test %s:\nExpected error containing '%s'\nGot: %v", tt.name, tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("Test %s:\nUnexpected error: %v", tt.name, err)
			}

			// For successful run, the exact time string (0 seconds ago) is hard to match.
			// So, for successful runs, just check if the significant parts are present.
			// For no repos found, also check with Contains.
			if tt.expectedOutput != "" && tt.expectedError == "" {
				if !strings.Contains(outputBuf.String(), tt.expectedOutput) {
					t.Errorf("Test %s:\nExpected output to contain '%s', Got:\n%q", tt.name, tt.expectedOutput, outputBuf.String())
				}
				// Additional checks for specific success/no-repo outputs
				if strings.Contains(tt.name, "Successful run") {
					if !strings.Contains(outputBuf.String(), "dummy-repo") || !strings.Contains(outputBuf.String(), "Initial commit") {
						t.Errorf("Test %s:\nSuccessful run output missing key elements. Got:\n%q", tt.name, outputBuf.String())
					}
				} else if strings.Contains(tt.name, "No repositories found") {
					if !strings.Contains(outputBuf.String(), "No recent commits found in any repository.") {
						t.Errorf("Test %s:\nNo repositories found output missing key elements. Got:\n%q", tt.name, outputBuf.String())
					}
				}
			} else if outputBuf.String() != tt.expectedOutput { // For exact match when expecting empty output for error cases
				t.Errorf("Test %s:\nExpected output:\n%q\nGot:\n%q", tt.name, tt.expectedOutput, outputBuf.String())
			}

			// Check error log output if expected
			if tt.expectedLog != "" {
				if !strings.Contains(errorBuf.String(), tt.expectedLog) {
					t.Errorf("Test %s:\nExpected error log to contain '%s', Got:\n%q", tt.name, tt.expectedLog, errorBuf.String())
				}
			} else if errorBuf.Len() > 0 {
				t.Logf("Test %s:\nUnexpected error log output (slog):\n%s", tt.name, errorBuf.String())
			}
		})
	}
}
