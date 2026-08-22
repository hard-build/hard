package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const artifactCacheVersion = 2

const (
	buildCacheSuffix      = ".hard-cache.json"
	parseCacheSuffix      = ".hard-parse-cache.json"
	testResultCacheSuffix = ".hard-test-cache.json"
)

type cacheFile struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type cacheAction struct {
	Version          int         `json:"version"`
	Kind             string      `json:"kind"`
	Hard             string      `json:"hard"`
	Tool             *cacheFile  `json:"tool,omitempty"`
	WorkingDirectory string      `json:"working_directory"`
	Arguments        []string    `json:"arguments"`
	Inputs           []cacheFile `json:"inputs"`
}

type cacheRecord struct {
	Version  int    `json:"version"`
	Input    string `json:"input"`
	Artifact string `json:"artifact"`
}

type parseCacheRecord struct {
	Version             int      `json:"version"`
	Kind                string   `json:"kind"`
	Input               string   `json:"input"`
	Result              string   `json:"result"`
	Dependencies        []string `json:"dependencies"`
	ManagedDependencies []string `json:"managed_dependencies,omitempty"`
	EntryPoint          string   `json:"entry_point,omitempty"`
	Forward             string   `json:"forward,omitempty"`
}

type digestResult struct {
	path   string
	digest string
	err    error
}

type artifactCache struct {
	read        bool
	hard        string
	mu          sync.Mutex
	fileDigests map[string]digestResult
	toolDigests map[string]digestResult
}

func newArtifactCache(read bool) (*artifactCache, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate hard executable for cache: %w", err)
	}
	hard, err := digestInputFile(executable)
	if err != nil {
		return nil, fmt.Errorf("fingerprint hard executable for cache: %w", err)
	}
	return &artifactCache{
		read:        read,
		hard:        hard,
		fileDigests: make(map[string]digestResult),
		toolDigests: make(map[string]digestResult),
	}, nil
}

func (cache *artifactCache) actionFingerprint(
	kind string,
	tool string,
	arguments []string,
	inputs []string,
	workingDirectory string,
) (string, error) {
	action := cacheAction{
		Version:          artifactCacheVersion,
		Kind:             kind,
		Hard:             cache.hard,
		WorkingDirectory: workingDirectory,
		Arguments:        append([]string(nil), arguments...),
	}
	if tool != "" {
		fingerprint, err := cache.toolFingerprint(tool, workingDirectory)
		if err != nil {
			return "", err
		}
		action.Tool = &fingerprint
	}

	files := make(map[string]string)
	for _, path := range inputs {
		canonical, digest, err := cache.inputFingerprint(path, workingDirectory)
		if err != nil {
			return "", err
		}
		files[canonical] = digest
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	action.Inputs = make([]cacheFile, 0, len(paths))
	for _, path := range paths {
		action.Inputs = append(action.Inputs, cacheFile{
			Path:   path,
			Digest: files[path],
		})
	}

	encoded, err := json.Marshal(action)
	if err != nil {
		return "", fmt.Errorf("encode %s cache input: %w", kind, err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (cache *artifactCache) inputFingerprint(
	path string,
	workingDirectory string,
) (string, string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve cache input %s: %w", path, err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", "", fmt.Errorf("make cache input absolute %s: %w", path, err)
	}

	cache.mu.Lock()
	result, ok := cache.fileDigests[canonical]
	cache.mu.Unlock()
	if !ok {
		result.digest, result.err = digestInputFile(canonical)
		cache.mu.Lock()
		cache.fileDigests[canonical] = result
		cache.mu.Unlock()
	}
	if result.err != nil {
		return "", "", fmt.Errorf("fingerprint cache input %s: %w", canonical, result.err)
	}
	return filepath.Clean(canonical), result.digest, nil
}

func (cache *artifactCache) toolFingerprint(
	tool string,
	workingDirectory string,
) (cacheFile, error) {
	key := workingDirectory + "\x00" + tool
	cache.mu.Lock()
	result, ok := cache.toolDigests[key]
	cache.mu.Unlock()
	if ok {
		if result.err != nil {
			return cacheFile{}, result.err
		}
		return cacheFile{Path: result.path, Digest: result.digest}, nil
	}

	path, err := exec.LookPath(tool)
	if err == nil && !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	if err == nil {
		path, err = filepath.EvalSymlinks(path)
	}
	if err == nil {
		path, err = filepath.Abs(path)
	}
	if err == nil {
		result.digest, result.err = digestInputFile(path)
		result.path = filepath.Clean(path)
	} else {
		result.err = fmt.Errorf("locate cache tool %s: %w", tool, err)
	}
	cache.mu.Lock()
	cache.toolDigests[key] = result
	cache.mu.Unlock()
	if result.err != nil {
		return cacheFile{}, result.err
	}
	return cacheFile{Path: result.path, Digest: result.digest}, nil
}

func (cache *artifactCache) hit(
	artifact string,
	suffix string,
	input string,
) (bool, error) {
	if !cache.read {
		return false, nil
	}
	record, ok, err := readCacheRecord(artifact + suffix)
	if err != nil || !ok || record.Version != artifactCacheVersion || record.Input != input {
		return false, err
	}
	digest, ok, err := digestArtifact(artifact)
	if err != nil || !ok {
		return false, err
	}
	return digest == record.Artifact, nil
}

func (cache *artifactCache) invalidate(artifact string, suffix string) error {
	err := os.Remove(artifact + suffix)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (cache *artifactCache) store(
	artifact string,
	suffix string,
	input string,
) error {
	digest, ok, err := digestArtifact(artifact)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("cache artifact is not a regular file: %s", artifact)
	}
	record := cacheRecord{
		Version:  artifactCacheVersion,
		Input:    input,
		Artifact: digest,
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode cache record for %s: %w", artifact, err)
	}
	encoded = append(encoded, '\n')
	return writeCacheRecord(artifact+suffix, encoded)
}

func parseCacheArguments(cflags []string, entryPoints []string) []string {
	arguments := []string{"libclang:" + clangVersion()}
	for _, cflag := range cflags {
		arguments = append(arguments, "cflag:"+cflag)
	}
	for _, entryPoint := range entryPoints {
		arguments = append(arguments, "entry-point:"+entryPoint)
	}
	return arguments
}

func parseCachePath(root, environment, source string) (string, error) {
	object, err := objectFilePath(root, environment, source)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(object, ".o") + parseCacheSuffix, nil
}

func (cache *artifactCache) parseHit(
	path string,
	kind string,
	source string,
	arguments []string,
	workingDirectory string,
) (parseCacheRecord, bool, error) {
	if !cache.read {
		return parseCacheRecord{}, false, nil
	}
	record, ok, err := readParseCacheRecord(path)
	if err != nil || !ok || record.Version != artifactCacheVersion || record.Kind != kind {
		return parseCacheRecord{}, false, err
	}
	inputs := append([]string{source}, record.Dependencies...)
	input, err := cache.actionFingerprint(kind, "", arguments, inputs, workingDirectory)
	if err != nil || input != record.Input {
		return parseCacheRecord{}, false, nil
	}
	result, err := parseResultFingerprint(record)
	if err != nil || result != record.Result {
		return parseCacheRecord{}, false, nil
	}
	return record, true, nil
}

func (cache *artifactCache) invalidateParse(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (cache *artifactCache) storeParse(
	path string,
	record parseCacheRecord,
	source string,
	arguments []string,
	workingDirectory string,
) (bool, error) {
	unsafe, err := parseCacheInputContainsHasInclude(source, arguments, workingDirectory)
	if err != nil {
		return false, err
	}
	if unsafe {
		return false, nil
	}
	inputs := append([]string{source}, record.Dependencies...)
	input, err := cache.actionFingerprint(record.Kind, "", arguments, inputs, workingDirectory)
	if err != nil {
		return false, err
	}
	record.Version = artifactCacheVersion
	record.Input = input
	record.Result, err = parseResultFingerprint(record)
	if err != nil {
		return false, err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("encode parse cache record for %s: %w", source, err)
	}
	encoded = append(encoded, '\n')
	if err := writeCacheRecord(path, encoded); err != nil {
		return false, err
	}
	return true, nil
}

func parseCacheInputContainsHasInclude(
	input string,
	arguments []string,
	workingDirectory string,
) (bool, error) {
	for _, argument := range arguments {
		if strings.Contains(argument, "__has_include") {
			return true, nil
		}
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, fmt.Errorf("resolve parse cache input %s: %w", path, err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return false, fmt.Errorf("make parse cache input absolute %s: %w", path, err)
	}
	contents, err := os.ReadFile(filepath.Clean(canonical))
	if err != nil {
		return false, fmt.Errorf("read parse cache input %s: %w", canonical, err)
	}
	return bytes.Contains(contents, []byte("__has_include")), nil
}

func parseResultFingerprint(record parseCacheRecord) (string, error) {
	result := struct {
		Kind                string   `json:"kind"`
		Dependencies        []string `json:"dependencies"`
		ManagedDependencies []string `json:"managed_dependencies,omitempty"`
		EntryPoint          string   `json:"entry_point,omitempty"`
		Forward             string   `json:"forward,omitempty"`
	}{
		Kind:                record.Kind,
		Dependencies:        record.Dependencies,
		ManagedDependencies: record.ManagedDependencies,
		EntryPoint:          record.EntryPoint,
		Forward:             record.Forward,
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode parse cache result: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func readParseCacheRecord(path string) (parseCacheRecord, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return parseCacheRecord{}, false, nil
	}
	if err != nil {
		return parseCacheRecord{}, false, fmt.Errorf("inspect parse cache record %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return parseCacheRecord{}, false, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return parseCacheRecord{}, false, fmt.Errorf("read parse cache record %s: %w", path, err)
	}
	var record parseCacheRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		return parseCacheRecord{}, false, nil
	}
	return record, true, nil
}

func readCacheRecord(path string) (cacheRecord, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return cacheRecord{}, false, nil
	}
	if err != nil {
		return cacheRecord{}, false, fmt.Errorf("inspect cache record %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return cacheRecord{}, false, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return cacheRecord{}, false, fmt.Errorf("read cache record %s: %w", path, err)
	}
	var record cacheRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		return cacheRecord{}, false, nil
	}
	return record, true, nil
}

func writeCacheRecord(path string, contents []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create cache directory %s: %w", directory, err)
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("create temporary cache record for %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	removeTemporary = false
	return nil
}

func digestInputFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file")
	}
	return digestReader(file)
}

func digestArtifact(path string) (string, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("inspect cache artifact %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", false, fmt.Errorf("open cache artifact %s: %w", path, err)
	}
	defer file.Close()
	digest, err := digestReader(file)
	if err != nil {
		return "", false, fmt.Errorf("fingerprint cache artifact %s: %w", path, err)
	}
	return digest, true, nil
}

func digestReader(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sameRegularFiles(left string, right string) (bool, error) {
	leftInfo, err := os.Lstat(left)
	if err != nil {
		return false, fmt.Errorf("inspect source artifact %s: %w", left, err)
	}
	if !leftInfo.Mode().IsRegular() {
		return false, fmt.Errorf("source artifact is not a regular file: %s", left)
	}
	rightInfo, err := os.Lstat(right)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect destination artifact %s: %w", right, err)
	}
	if !rightInfo.Mode().IsRegular() || leftInfo.Mode().Perm() != rightInfo.Mode().Perm() {
		return false, nil
	}
	leftDigest, _, err := digestArtifact(left)
	if err != nil {
		return false, err
	}
	rightDigest, _, err := digestArtifact(right)
	if err != nil {
		return false, err
	}
	return leftDigest == rightDigest, nil
}
