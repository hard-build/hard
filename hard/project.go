package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"go.yaml.in/yaml/v3"
)

const projectFilename = "hard.yaml"

type projectOptions struct {
	explicitFormat bool
	lock           bool
	locked         bool
	updates        []string
}

type repositoryPin struct {
	Source   string `yaml:"source" json:"source"`
	Ref      string `yaml:"ref" json:"ref"`
	Commit   string `yaml:"commit" json:"commit"`
	Checksum string `yaml:"checksum" json:"checksum"`
}

type projectFile struct {
	Version      int                      `yaml:"version"`
	Format       string                   `yaml:"format"`
	Exclude      []string                 `yaml:"exclude"`
	Repositories map[string]repositoryPin `yaml:"repositories"`

	filename string
	original []byte
	document yaml.Node
	recorded bool
}

func findProjectFile(directory string) (string, error) {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	for {
		filename := filepath.Join(directory, projectFilename)
		if _, err := os.Lstat(filename); err == nil {
			return filename, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if _, err := os.Lstat(filepath.Join(directory, ".git")); err == nil {
			return "", nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", nil
		}
		directory = parent
	}
}

func readProjectFile(filename string) (*projectFile, error) {
	contents, err := readRegularProjectFile(filename)
	if err != nil {
		return nil, err
	}
	project := &projectFile{filename: filename, original: contents}
	if err := decodeConfigurationYAML(contents, project, &project.document); err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	if project.Version != 1 {
		return nil, fmt.Errorf("%s: version must be 1", filename)
	}
	root := project.document.Content[0]
	if root.Style&yaml.FlowStyle != 0 {
		return nil, fmt.Errorf("%s: top-level configuration must use block mapping style", filename)
	}
	for index := 0; index < len(root.Content); index += 2 {
		key, value := root.Content[index].Value, root.Content[index+1]
		switch key {
		case "version":
			if value.Tag != "!!int" {
				return nil, fmt.Errorf("%s: version must be an integer", filename)
			}
		case "format":
			if value.Tag != "!!str" || project.Format == "" {
				return nil, fmt.Errorf("%s: format must be a non-empty string", filename)
			}
		case "exclude":
			if value.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("%s: exclude must be a list", filename)
			}
			for _, path := range value.Content {
				if path.Tag != "!!str" || validateProjectPath(path.Value) != nil {
					return nil, fmt.Errorf("%s: invalid %s path %q", filename, key, path.Value)
				}
			}
		case "repositories":
			if value.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("%s: repositories must be a mapping", filename)
			}
			project.recorded = true
			for entry := 1; entry < len(value.Content); entry += 2 {
				pin := value.Content[entry]
				if pin.Kind != yaml.MappingNode {
					return nil, fmt.Errorf("%s: repository entry must be a mapping", filename)
				}
				for field := 1; field < len(pin.Content); field += 2 {
					if pin.Content[field].Tag != "!!str" {
						return nil, fmt.Errorf("%s: repository fields must be strings", filename)
					}
				}
			}
		}
	}
	for name, pin := range project.Repositories {
		if err := validateRepositoryPin(name, pin); err != nil {
			return nil, fmt.Errorf("%s: %w", filename, err)
		}
	}
	return project, nil
}

func decodeConfigurationYAML(contents []byte, target any, document *yaml.Node) error {
	if err := validateLibraryYAML(contents); err != nil {
		return err
	}
	if err := yaml.Unmarshal(contents, document); err != nil {
		return err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return errors.New("configuration must be a YAML mapping")
	}
	if err := validateConfigurationKeys(document); err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	return decoder.Decode(target)
}

func validateConfigurationKeys(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Tag != "!!str" || seen[key.Value] {
				return fmt.Errorf("non-string or duplicate YAML key %q", key.Value)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := validateConfigurationKeys(child); err != nil {
			return err
		}
	}
	return nil
}

func validateProjectPath(path string) error {
	if strings.ContainsAny(path, "\\*?[]\x00\r\n") {
		return errors.New("path must be literal")
	}
	return validateLibraryRelativePath(strings.TrimSuffix(path, "/"), true)
}

func readRegularProjectFile(filename string) ([]byte, error) {
	info, err := os.Lstat(filename)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("configuration is not a regular file: %s", filename)
	}
	return os.ReadFile(filename)
}

func (project *projectFile) sourcePaths(parsed arguments) ([]string, []string) {
	paths := parsed.paths
	var excluded []string
	if project == nil {
		return paths, excluded
	}
	directory := filepath.Dir(project.filename)
	for _, path := range project.Exclude {
		excluded = append(excluded, filepath.Join(directory, path))
	}
	return paths, excluded
}

// Lock the directory inode, not hard.yaml: atomic replacement changes the file inode.
// No auxiliary lock file is created in the source tree.
func lockProjectDirectory(filename string) (*os.File, error) {
	directory, err := os.Open(filepath.Dir(filename))
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(directory.Fd()), syscall.LOCK_EX); err != nil {
		_ = directory.Close()
		return nil, fmt.Errorf("lock project directory: %w", err)
	}
	return directory, nil
}

func (project *projectFile) writeRepositories(pins map[string]repositoryPin) error {
	current, err := readRegularProjectFile(project.filename)
	if errors.Is(err, os.ErrNotExist) && project.original == nil {
		current, err = nil, nil
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(current, project.original) {
		return fmt.Errorf("%s changed during dependency resolution; retry", project.filename)
	}
	root := project.document.Content[0]
	var repositories *yaml.Node
	var repositoryKey *yaml.Node
	start, end := -1, len(project.original)
	lines := bytes.SplitAfter(project.original, []byte("\n"))
	offset := func(line int) int {
		result := 0
		for index := 0; index < line && index < len(lines); index++ {
			result += len(lines[index])
		}
		return result
	}
	// Keep an explicit document-end marker outside the rewritten section, and
	// insert a newly added section before it instead of starting another document.
	for index, line := range lines {
		if bytes.HasPrefix(line, []byte("...")) {
			rest := strings.TrimSpace(string(line[3:]))
			if rest == "" || strings.HasPrefix(rest, "#") {
				end = offset(index)
				break
			}
		}
	}
	for index := 0; index < len(root.Content); index += 2 {
		if root.Content[index].Value == "repositories" {
			repositoryKey = root.Content[index]
			repositories = root.Content[index+1]
			start = offset(repositoryKey.Line - 1)
			if index+2 < len(root.Content) {
				next := root.Content[index+2]
				line := next.Line - 1
				if next.HeadComment != "" {
					line -= len(strings.Split(next.HeadComment, "\n"))
				}
				for line > repositoryKey.Line && len(bytes.TrimSpace(lines[line-1])) == 0 {
					line--
				}
				end = offset(line)
			}
		}
	}
	if repositories == nil {
		repositories = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "repositories"}, repositories)
	}
	repositories.Style = 0
	names := make([]string, 0, len(pins))
	for name := range pins {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		var existing *yaml.Node
		for index := 0; index < len(repositories.Content); index += 2 {
			if repositories.Content[index].Value == name {
				existing = repositories.Content[index+1]
			}
		}
		var value yaml.Node
		if err := value.Encode(pins[name]); err != nil {
			return err
		}
		if existing == nil {
			repositories.Content = append(repositories.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}, &value)
		} else {
			// Preserve comments and spelling on unchanged fields as well as the entry.
			for index := 0; index < len(value.Content); index += 2 {
				for old := 0; old < len(existing.Content); old += 2 {
					if existing.Content[old].Value == value.Content[index].Value {
						existing.Content[old+1].Value = value.Content[index+1].Value
					}
				}
			}
		}
	}
	var encoded bytes.Buffer
	encoder := yaml.NewEncoder(&encoded)
	encoder.SetIndent(2)
	if err := encoder.Encode(&project.document); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	if project.original != nil {
		// Serialize only the machine-owned section. All other bytes stay untouched.
		key := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "repositories"}
		if repositoryKey != nil {
			key = *repositoryKey
			key.HeadComment = "" // Its original lines are in the untouched prefix.
		}
		section := yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{&key, repositories}}
		encoded.Reset()
		encoder = yaml.NewEncoder(&encoded)
		encoder.SetIndent(2)
		if err := encoder.Encode(&section); err != nil {
			return err
		}
		if err := encoder.Close(); err != nil {
			return err
		}
		if start < 0 {
			start = end
		}
		result := append([]byte(nil), project.original[:start]...)
		if len(result) != 0 && result[len(result)-1] != '\n' {
			result = append(result, '\n')
		}
		result = append(result, encoded.Bytes()...)
		result = append(result, project.original[end:]...)
		return atomicProjectWrite(project.filename, result)
	}
	return atomicProjectWrite(project.filename, encoded.Bytes())
}

func atomicProjectWrite(filename string, contents []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Lstat(filename); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refuse to replace non-regular project file: %s", filename)
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".hard.yaml.*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	_, writeErr := temporary.Write(contents)
	if writeErr == nil {
		writeErr = temporary.Chmod(mode)
	}
	if writeErr == nil {
		writeErr = temporary.Sync()
	}
	if err := errors.Join(writeErr, temporary.Close()); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), filename)
}

// Resolution attempts may be retried. Only a completed attempt exposes diagnostics;
// compilation and child processes retain live stderr after resolution succeeds.
type resolutionWriter struct {
	buffer bytes.Buffer
	target io.Writer
	live   bool
}

func (writer *resolutionWriter) Write(contents []byte) (int, error) {
	if writer.live {
		return writer.target.Write(contents)
	}
	return writer.buffer.Write(contents)
}

func (writer *resolutionWriter) activate() error {
	writer.live = true
	_, err := io.Copy(writer.target, &writer.buffer)
	return err
}
