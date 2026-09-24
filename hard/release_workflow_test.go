package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func releasePublishScript(t *testing.T) string {
	t.Helper()
	contents, err := os.ReadFile("../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string
				Run  string
			}
		}
	}
	if err := yaml.Unmarshal(contents, &workflow); err != nil {
		t.Fatal(err)
	}
	var publish string
	for name, job := range workflow.Jobs {
		for _, step := range job.Steps {
			if step.Run == "" {
				continue
			}
			command := exec.Command("bash", "-n")
			command.Stdin = strings.NewReader(step.Run)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("%s/%s: %v\n%s", name, step.Name, err, output)
			}
			if name == "publish" && step.Name == "Publish GitHub release" {
				publish = step.Run
			}
		}
	}
	if publish == "" {
		t.Fatal("release publication step is missing")
	}
	return publish
}

func TestReleasePublishWorkflow(t *testing.T) {
	script := releasePublishScript(t)
	const archive = "hard-v8.0.tar.gz"
	const checksum = archive + ".sha256"
	for _, test := range []struct {
		name       string
		mode       string
		draft      string
		existing   []string
		mismatch   bool
		wantError  string
		wantUpload int
	}{
		{name: "new release", wantUpload: 2},
		{name: "partial draft", draft: "true", existing: []string{archive}, wantUpload: 1},
		{name: "complete draft", draft: "true", existing: []string{archive, checksum}},
		{name: "published release", draft: "false", existing: []string{archive, checksum}},
		{name: "partial published release", draft: "false", existing: []string{archive}, wantUpload: 1},
		{name: "checksum accepted before 422", mode: "accepted-error", wantUpload: 2},
		{name: "checksum accepted before network error", mode: "accepted-network-error", wantUpload: 2},
		{name: "draft created before response lost", mode: "create-response-lost", wantUpload: 2},
		{name: "checksum becomes visible later", mode: "delayed-download", wantUpload: 2},
		{name: "existing checksum differs", draft: "true", existing: []string{checksum}, mismatch: true, wantError: "different contents"},
		{name: "published checksum differs", draft: "false", existing: []string{checksum}, mismatch: true, wantError: "different contents"},
		{name: "conflicting upload", mode: "conflicting-upload", wantUpload: 2, wantError: "different contents"},
		{name: "failed upload", mode: "failed-upload", wantUpload: 2, wantError: "HTTP 503"},
		{name: "download unavailable", mode: "failed-download", draft: "true", existing: []string{archive, checksum}, wantError: "download unavailable"},
		{name: "lookup denied", mode: "view-denied", wantError: "HTTP 403"},
		{name: "lookup network failure", mode: "view-network-error", wantError: "connection reset"},
		{name: "creation failed", mode: "create-failed", wantError: "HTTP 403"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "bin")
			remote := filepath.Join(root, "remote")
			for _, directory := range []string{bin, remote, filepath.Join(root, "release")} {
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, asset := range []string{archive, checksum} {
				writeBuildFile(t, root, "release/"+asset, "contents of "+asset+"\n")
			}
			if test.draft != "" {
				writeBuildFile(t, remote, "draft", test.draft+"\n")
			}
			for _, asset := range test.existing {
				contents := readTestFile(t, filepath.Join(root, "release", asset))
				if test.mismatch {
					contents = "different contents\n"
				}
				writeBuildFile(t, remote, asset, contents)
			}
			writeWrapperExecutable(t, filepath.Join(bin, "gh"), releaseTestGitHubCLI)
			writeWrapperExecutable(t, filepath.Join(bin, "sleep"), "#!/bin/sh\nexit 0\n")
			log := filepath.Join(root, "requests")
			environment := wrapperTestEnvironment(map[string]string{
				"PATH":              bin + string(os.PathListSeparator) + os.Getenv("PATH"),
				"RUNNER_TEMP":       root,
				"TMPDIR":            root,
				"ARCHIVE_NAME":      archive,
				"CHECKSUM_NAME":     checksum,
				"RELEASE_TAG":       "v8.0",
				"GITHUB_REPOSITORY": "hard-build/hard",
				"RELEASE_TEST_ROOT": remote,
				"RELEASE_TEST_LOG":  log,
				"RELEASE_TEST_MODE": test.mode,
			})
			run := func(environment []string) ([]byte, error) {
				command := exec.Command("bash", "-c", script)
				command.Env = environment
				return command.CombinedOutput()
			}
			output, err := run(environment)
			if test.wantError != "" {
				if err == nil || !strings.Contains(string(output), test.wantError) {
					t.Fatalf("error = %v, want %q\n%s", err, test.wantError, output)
				}
			} else if err != nil {
				t.Fatalf("publish: %v\n%s", err, output)
			}
			requests := readTestFile(t, log)
			if count := strings.Count(requests, "upload "); count != test.wantUpload {
				t.Fatalf("uploads = %d, want %d\n%s", count, test.wantUpload, requests)
			}
			if test.wantError != "" {
				if strings.Contains(requests, "publish\n") {
					t.Fatalf("published after an error:\n%s", requests)
				}
				if strings.HasPrefix(test.mode, "view-") && strings.Contains(requests, "create\n") {
					t.Fatalf("created a release after a lookup error:\n%s", requests)
				}
				if test.mismatch && readTestFile(t, filepath.Join(remote, checksum)) != "different contents\n" {
					t.Fatal("overwrote an existing asset")
				}
				if test.mode != "failed-upload" {
					return
				}
				if readTestFile(t, filepath.Join(remote, "draft")) != "true\n" {
					t.Fatal("failed upload did not retain its draft")
				}
				// Resume the same draft with its accepted archive after the outage.
				for index, value := range environment {
					if strings.HasPrefix(value, "RELEASE_TEST_MODE=") {
						environment[index] = "RELEASE_TEST_MODE="
					}
				}
			} else {
				if readTestFile(t, filepath.Join(remote, "draft")) != "false\n" {
					t.Fatal("verified release was not published")
				}
				if test.draft == "false" && strings.Contains(requests, "publish\n") {
					t.Fatal("modified an already published release")
				}
			}
			if err := os.WriteFile(log, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if output, err := run(environment); err != nil {
				t.Fatalf("retry: %v\n%s", err, output)
			}
			retry := readTestFile(t, log)
			wantUploads := 0
			if test.mode == "failed-upload" {
				wantUploads = 1
			}
			if strings.Count(retry, "upload ") != wantUploads || strings.Contains(retry, "create\n") {
				t.Fatalf("retry did not reuse accepted files:\n%s", retry)
			}
			if readTestFile(t, filepath.Join(remote, "draft")) != "false\n" {
				t.Fatal("retry left a draft")
			}
		})
	}
}

const releaseTestGitHubCLI = `#!/bin/bash
set -euo pipefail
fail() { echo "$*" >&2; exit 1; }
[[ $1 == release && $3 == "$RELEASE_TAG" ]] || fail "unexpected command: $*"
action=$2
shift 3
repo= json= pattern= directory= draft=false verify=false
files=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) repo=$2; shift 2 ;;
    --json) json=$2; shift 2 ;;
    --jq|--title) shift 2 ;;
    --pattern) pattern=$2; shift 2 ;;
    --dir) directory=$2; shift 2 ;;
    --draft) draft=true; shift ;;
    --verify-tag) verify=true; shift ;;
    --draft=false|--generate-notes) shift ;;
    --*) fail "unexpected flag: $1" ;;
    *) files+=("$1"); shift ;;
  esac
done
[[ $repo == "$GITHUB_REPOSITORY" ]] || fail "missing repository"
root=$RELEASE_TEST_ROOT
mode=$RELEASE_TEST_MODE
case "$action" in
  view)
    echo view >> "$RELEASE_TEST_LOG"
    [[ $mode != view-denied ]] || fail "HTTP 403: denied"
    [[ $mode != view-network-error ]] || fail "connection reset"
    [[ -f $root/draft ]] || fail "release not found"
    case "$json" in
      isDraft) cat "$root/draft" ;;
      assets)
        for name in "$ARCHIVE_NAME" "$CHECKSUM_NAME"; do
          if [[ -f $root/$name ]]; then echo "$name"; fi
        done ;;
      *) fail "unexpected view: $json" ;;
    esac ;;
  create)
    echo create >> "$RELEASE_TEST_LOG"
    [[ $draft == true && $verify == true && ${#files[@]} == 0 ]] || fail "unsafe release creation"
    [[ $mode != create-failed ]] || fail "HTTP 403: denied"
    echo true > "$root/draft"
    [[ $mode != create-response-lost ]] || fail "connection reset" ;;
  upload)
    [[ ${#files[@]} == 1 ]] || fail "expected one asset per upload"
    name=$(basename "${files[0]}")
    echo "upload $name" >> "$RELEASE_TEST_LOG"
    [[ ! -f $root/$name ]] || fail "attempted to overwrite asset"
    if [[ $name == "$CHECKSUM_NAME" && $mode == failed-upload ]]; then
      fail "HTTP 503: upload unavailable"
    fi
    cp "${files[0]}" "$root/$name"
    if [[ $name == "$CHECKSUM_NAME" ]]; then
      case "$mode" in
        accepted-error) fail "HTTP 422: ReleaseAsset.name already exists" ;;
        accepted-network-error) fail "connection reset" ;;
        conflicting-upload)
          echo "different contents" > "$root/$name"
          fail "HTTP 422: ReleaseAsset.name already exists" ;;
      esac
    fi ;;
  download)
    echo "download $pattern" >> "$RELEASE_TEST_LOG"
    [[ $mode != failed-download ]] || fail "download unavailable"
    [[ -f $root/$pattern ]] || fail "asset not found: $pattern"
    if [[ $mode == delayed-download && $pattern == "$CHECKSUM_NAME" ]]; then
      count=0
      if [[ -f $root/download-count ]]; then count=$(cat "$root/download-count"); fi
      echo $((count + 1)) > "$root/download-count"
      [[ $count -ge 2 ]] || fail "asset not yet visible"
    fi
    cp "$root/$pattern" "$directory/$pattern" ;;
  edit)
    [[ $verify == true ]] || fail "missing tag verification"
    for name in "$ARCHIVE_NAME" "$CHECKSUM_NAME"; do
      cmp "$root/$name" "$RUNNER_TEMP/release/$name" || fail "publishing incomplete release"
    done
    echo publish >> "$RELEASE_TEST_LOG"
    echo false > "$root/draft" ;;
  *) fail "unexpected action: $action" ;;
esac
`
