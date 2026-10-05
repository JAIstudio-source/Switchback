package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseHookStdin(t *testing.T) {
	// Case 1: Root tool_input
	json1 := `{"tool_name":"ask_question","tool_input":{"questions":[{"question":"Run build?","options":["Yes","No"]}]},"session_id":"test-session-1"}`
	toolName, toolInput, _, convID := parseHookStdin([]byte(json1))

	if toolName != "ask_question" {
		t.Errorf("Expected toolName ask_question, got %s", toolName)
	}
	if convID != "test-session-1" {
		t.Errorf("Expected convID test-session-1, got %s", convID)
	}
	if toolInput == nil {
		t.Fatalf("Expected non-nil toolInput")
	}

	// Case 2: Nested toolObj with parameters
	json2 := `{"tool":{"name":"run_command","parameters":{"CommandLine":"npm test"}}}`
	toolName2, toolInput2, _, _ := parseHookStdin([]byte(json2))
	if toolName2 != "run_command" {
		t.Errorf("Expected toolName run_command, got %s", toolName2)
	}
	if toolInput2 == nil || toolInput2["CommandLine"] != "npm test" {
		t.Errorf("Expected CommandLine npm test in toolInput, got %+v", toolInput2)
	}

	// Case 3: Empty JSON fallback
	json3 := `{}`
	toolName3, toolInput3, _, _ := parseHookStdin([]byte(json3))
	if toolName3 != "" || toolInput3 != nil {
		t.Errorf("Expected empty result for {}, got %s, %+v", toolName3, toolInput3)
	}
}

func TestParseApprovalData(t *testing.T) {
	// Case 1: ask_question tool
	toolInput := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question": "Which database would you like to use?",
				"options":  []interface{}{"PostgreSQL", "SQLite", "MySQL"},
			},
		},
	}
	title, msg, options, toolClean, snippet, dangerLevel := parseApprovalData("ask_question", toolInput, "")
	if title != "AI Agent Question" {
		t.Errorf("Expected title 'AI Agent Question', got %s", title)
	}
	if msg != "Which database would you like to use?" {
		t.Errorf("Expected message 'Which database would you like to use?', got %s", msg)
	}
	if len(options) != 3 || options[0] != "PostgreSQL" {
		t.Errorf("Expected 3 options starting with PostgreSQL, got %+v", options)
	}
	if toolClean != "ask_question" || snippet == "" || dangerLevel != "medium" {
		t.Errorf("Unexpected tool details: %s, %s, %s", toolClean, snippet, dangerLevel)
	}

	// Case 2: run_command tool (destructive)
	cmdInput := map[string]interface{}{
		"CommandLine": "rm -rf ./cache",
	}
	titleCmd, msgCmd, optionsCmd, toolCleanCmd, snippetCmd, dangerCmd := parseApprovalData("run_command", cmdInput, "")
	if !strings.Contains(titleCmd, "run_command") {
		t.Errorf("Expected title to mention run_command, got %s", titleCmd)
	}
	if !strings.Contains(msgCmd, "rm -rf ./cache") {
		t.Errorf("Expected msg to contain command string, got %s", msgCmd)
	}
	if toolCleanCmd != "run_command" || snippetCmd != "rm -rf ./cache" || dangerCmd != "danger" {
		t.Errorf("Expected danger level for rm -rf, got %s", dangerCmd)
	}
	expectedOpts := []string{"Allow Action", "Deny Action"}
	if !reflect.DeepEqual(optionsCmd, expectedOpts) {
		t.Errorf("Expected default allow/deny options, got %+v", optionsCmd)
	}
}

func TestIsSafeAction(t *testing.T) {
	// Safe read-only tools
	if !isSafeAction("read_file", map[string]interface{}{"AbsolutePath": "foo.go"}) {
		t.Errorf("read_file should be safe")
	}
	if !isSafeAction("list_dir", map[string]interface{}{"DirectoryPath": "src"}) {
		t.Errorf("list_dir should be safe")
	}
	if !isSafeAction("grep_search", map[string]interface{}{"Query": "func"}) {
		t.Errorf("grep_search should be safe")
	}

	// Safe command
	if !isSafeAction("run_command", map[string]interface{}{"CommandLine": "git status"}) {
		t.Errorf("git status should be safe")
	}
	if !isSafeAction("run_command", map[string]interface{}{"CommandLine": "ls -la"}) {
		t.Errorf("ls should be safe")
	}

	// Dangerous commands
	if isSafeAction("run_command", map[string]interface{}{"CommandLine": "rm -rf /"}) {
		t.Errorf("rm -rf should NOT be safe")
	}
	if isSafeAction("run_command", map[string]interface{}{"CommandLine": "git push origin main"}) {
		t.Errorf("git push should NOT be safe")
	}
	if isSafeAction("write_to_file", map[string]interface{}{"TargetFile": "main.go"}) {
		t.Errorf("write_to_file should NOT be safe")
	}

	// Safe ask_question
	safeQ := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question": "Would you like to review project status?",
			},
		},
	}
	if !isSafeAction("ask_question", safeQ) {
		t.Errorf("safe ask_question should be classified as safe")
	}

	// Destructive ask_question
	dangerQ := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question": "Do you want to delete and wipe the test folder?",
			},
		},
	}
	if isSafeAction("ask_question", dangerQ) {
		t.Errorf("destructive ask_question should NOT be safe")
	}
}

