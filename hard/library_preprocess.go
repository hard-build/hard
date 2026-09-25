package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// The ordinary dependency analysis keeps going past absent vendor headers.
// For the bootstrap preprocessing pass only, missing non-recipe includes have
// empty placeholders. Once package includes are available the normal analysis
// and this pass run again with the real headers, before compiling a consumer.
// Fetch does not need package variants and never invokes the compiler.
type libraryVisit struct {
	Header       string
	Context      string
	Dependencies []int
}

func (manager *libraryManager) preprocessLibraryVisits(source string, flags, headers, unresolved []string, includes []clangInclude) ([]libraryVisit, error) {
	contexts := make(map[string]bool, len(headers))
	for _, header := range headers {
		contexts[header] = true
	}
	arguments := append([]string(nil), flags...)
	if len(unresolved) > 0 {
		directory, err := os.MkdirTemp("", "hard-recipe-headers-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(directory)
		for _, include := range unresolved {
			if isLibraryHeader(include) {
				return nil, fmt.Errorf("recipe header is unavailable: %s", include)
			}
			path := filepath.Join(directory, filepath.FromSlash(include))
			if filepath.IsAbs(include) || !pathWithin(directory, path) {
				return nil, fmt.Errorf("cannot bootstrap unresolved include %s", include)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				return nil, err
			}
		}
		arguments = append(arguments, "-I"+directory)
	}
	language := false
	for _, argument := range arguments {
		if strings.HasPrefix(argument, "-x") {
			language = true
		}
	}
	if !language {
		arguments = append(arguments, "-x", "c++")
	}
	arguments = append(arguments, "-E", "-dI", source, "-o", "-")
	command := exec.Command(manager.compiler, arguments...)
	command.Dir = manager.workingDirectory
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("preprocess recipe headers in %s: %w\n%s", source, err, stderr.String())
	}
	return parseLibraryVisits(&stdout, manager.workingDirectory, contexts, includes)
}

// Keep each entry into a wrapper separate, including repeated, unguarded
// includes under different macros. -dI also reports includes skipped by guards;
// those retain the already selected variant and its dependency edges.
func parseLibraryVisits(output *bytes.Buffer, workingDirectory string, headers map[string]bool, includes []clangInclude) ([]libraryVisit, error) {
	targets := make(map[string]string)
	for _, include := range includes {
		if include.source != "" && include.target != "" {
			source, err := realAbsolutePath(include.source, workingDirectory)
			if err != nil {
				return nil, err
			}
			target, err := realAbsolutePath(include.target, workingDirectory)
			if err != nil {
				return nil, err
			}
			targets[source+"\x00"+include.spelling] = target
		}
	}
	type frame struct {
		file     string
		visit    int
		children []int
	}
	var visits []libraryVisit
	var stack []frame
	completed := make(map[string][]int)
	seen := make(map[string]bool)
	var pending string
	addSkipped := func() error {
		if pending == "" || len(stack) == 0 {
			return nil
		}
		parent := &stack[len(stack)-1]
		target := targets[parent.file+"\x00"+pending]
		pending = ""
		for _, active := range stack {
			if active.file == target && headers[target] {
				return fmt.Errorf("recipe dependency cycle through %s", target)
			}
		}
		if headers[target] && !seen[target] {
			// A guard can suppress the whole file, including its line markers.
			// The active include still selects its recipe with empty own code.
			completed[target] = []int{len(visits)}
			visits = append(visits, libraryVisit{Header: target})
			seen[target] = true
		}
		parent.children = append(parent.children, completed[target]...)
		return nil
	}
	closeFrame := func() {
		last := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		roots := last.children
		if last.visit >= 0 {
			visits[last.visit].Dependencies = last.children
			roots = []int{last.visit}
		}
		completed[last.file] = roots
		if len(stack) > 0 {
			parent := &stack[len(stack)-1]
			parent.children = append(parent.children, roots...)
		}
	}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 64*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if path, markerFlags, ok := preprocessorLineMarker(line); ok {
			if strings.HasPrefix(path, "<") {
				if strings.Contains(" "+markerFlags+" ", " 2 ") && len(stack) > 1 {
					closeFrame()
				}
				continue
			}
			canonical, err := realAbsolutePath(path, workingDirectory)
			if err != nil {
				return nil, err
			}
			if strings.Contains(" "+markerFlags+" ", " 1 ") || len(stack) == 0 {
				pending = "" // This include entered a file rather than being skipped.
				next := frame{file: canonical, visit: -1}
				if headers[canonical] {
					next.visit = len(visits)
					visits = append(visits, libraryVisit{Header: canonical})
					seen[canonical] = true
				}
				stack = append(stack, next)
			} else if strings.Contains(" "+markerFlags+" ", " 2 ") {
				if err := addSkipped(); err != nil {
					return nil, err
				}
				for len(stack) > 1 && stack[len(stack)-1].file != canonical {
					closeFrame()
				}
			}
			continue
		}
		if err := addSkipped(); err != nil {
			return nil, err
		}
		if strings.HasPrefix(line, "#include ") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "#include "))
			if len(rest) > 1 && (rest[0] == '"' || rest[0] == '<') {
				closing := byte('"')
				if rest[0] == '<' {
					closing = '>'
				}
				if end := strings.IndexByte(rest[1:], closing); end >= 0 {
					pending = rest[1 : end+1]
				}
			}
			continue
		}
		if len(stack) != 0 && strings.TrimSpace(line) != "" {
			if index := stack[len(stack)-1].visit; index >= 0 {
				visits[index].Context += line + "\n"
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read recipe preprocessing output: %w", err)
	}
	if err := addSkipped(); err != nil {
		return nil, err
	}
	for len(stack) > 0 {
		closeFrame()
	}
	for path := range headers {
		if !seen[path] {
			return nil, fmt.Errorf("preprocessor did not visit recipe header %s", path)
		}
	}
	for i := range visits {
		if visits[i].Context != "" {
			sum := sha256.Sum256([]byte(visits[i].Context))
			visits[i].Context = hex.EncodeToString(sum[:])
		}
	}
	return visits, nil
}

func preprocessorLineMarker(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "# ") {
		return "", "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "# "))
	position := strings.IndexByte(rest, ' ')
	if position < 0 {
		return "", "", false
	}
	if _, err := strconv.ParseUint(rest[:position], 10, 32); err != nil {
		return "", "", false
	}
	rest = strings.TrimSpace(rest[position:])
	if !strings.HasPrefix(rest, "\"") {
		return "", "", false
	}
	for i := 1; i < len(rest); i++ {
		if rest[i] == '\\' {
			i++
			continue
		}
		if rest[i] == '"' {
			path, err := strconv.Unquote(rest[:i+1])
			return path, strings.TrimSpace(rest[i+1:]), err == nil
		}
	}
	return "", "", false
}
