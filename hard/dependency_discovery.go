package main

import (
	"encoding/json"
	"path/filepath"
	"sync"
)

// This is an input-validated discovery hint, never a project dependency record.
// In particular, restored pins do not enter project.Repositories, so inherited
// requirements still apply when cached or fresh analysis visits include edges.
type dependencyDiscovery struct {
	key      string
	anchors  []string
	kind     string
	read     bool
	mutex    sync.Mutex
	inputs   map[string]string
	disabled bool
}

type dependencyDiscoveryRecord struct {
	Version int                      `json:"version"`
	Key     string                   `json:"key"`
	Pins    map[string]repositoryPin `json:"repositories"`
	Inputs  map[string]string        `json:"inputs"`
	Result  string                   `json:"result"`
}

func (session *dependencySession) prepareDiscoveryCache(configuration configuration, parsed arguments, sources []string) error {
	if session == nil || session.record || parsed.command == "format" || len(sources) == 0 {
		return nil
	}
	owner, err := localProjectRoot(session.root, configuration.env, session.workingDirectory)
	if err != nil {
		return err
	}
	// Before the first view, selected is empty: this key describes the invocation,
	// independently of the dependencies it will discover.
	layout, err := newCacheLayout(session, configuration, owner)
	if err != nil {
		return err
	}
	cache, err := newArtifactCache(!parsed.noCache)
	if err != nil {
		return err
	}
	context, err := json.Marshal(struct {
		Hard, Build, Command, Project string
		Configuration                 []byte
		Sources                       []string
	}{cache.hard, layout.buildKey, parsed.command, session.project.filename, session.project.original, sources})
	if err != nil {
		return err
	}
	kind := "source-parse"
	if parsed.command == "fetch" {
		kind = "fetch-parse"
	}
	discovery := &dependencyDiscovery{
		key: repositoryDigest(context), kind: kind,
		read: !parsed.noCache, inputs: make(map[string]string),
	}
	for _, source := range sources {
		var path string
		if kind == "fetch-parse" {
			path, err = fetchParseCachePath(session.root, configuration.env, source, layout)
		} else {
			path, err = parseCachePath(session.root, configuration.env, source, layout)
		}
		if err != nil {
			return err
		}
		discovery.anchors = append(discovery.anchors, path)
	}
	session.discovery = discovery
	return session.discovery.observe(cache, sources, session.workingDirectory)
}

func (discovery *dependencyDiscovery) observe(cache *artifactCache, inputs []string, directory string) error {
	if discovery == nil {
		return nil
	}
	for _, input := range inputs {
		_, digest, err := cache.inputFingerprint(input, directory)
		if err != nil {
			return err
		}
		path, err := lexicalAbsolutePath(input, directory)
		if err != nil {
			return err
		}
		discovery.mutex.Lock()
		discovery.inputs[path] = digest
		discovery.mutex.Unlock()
	}
	return nil
}

func (discovery *dependencyDiscovery) observeDefault(path string, contents []byte) {
	if discovery != nil {
		discovery.mutex.Lock()
		discovery.inputs[path] = repositoryDigest(contents)
		discovery.mutex.Unlock()
	}
}

func (discovery *dependencyDiscovery) disable() {
	if discovery != nil {
		discovery.mutex.Lock()
		discovery.disabled = true
		discovery.mutex.Unlock()
	}
}

func discoveryRecordDigest(record dependencyDiscoveryRecord) string {
	record.Result = ""
	contents, _ := json.Marshal(record)
	return repositoryDigest(contents)
}

func discoveryInputsMatch(inputs map[string]string) bool {
	for path, expected := range inputs {
		if !filepath.IsAbs(path) {
			return false
		}
		var digest string
		var err error
		if filepath.Base(path) == "@default" {
			var contents []byte
			contents, err = readRegularProjectFile(path)
			digest = repositoryDigest(contents)
		} else {
			digest, err = digestInputFile(path)
		}
		if err != nil || digest != expected {
			return false
		}
	}
	return true
}

// Called under the project/environment lock, before refreshing include links.
func (session *dependencySession) restoreDiscoveryCache() {
	discovery := session.discovery
	if discovery == nil || !discovery.read {
		return
	}
	for _, path := range discovery.anchors {
		parse, ok, err := readParseCacheRecord(path)
		if err != nil || !ok || parse.Version != artifactCacheVersion || parse.Kind != discovery.kind || parse.Discovery == nil {
			continue
		}
		result, err := parseResultFingerprint(parse)
		if err != nil || result != parse.Result {
			continue
		}
		record := parse.Discovery
		if record.Version != 1 || record.Key != discovery.key ||
			record.Result != discoveryRecordDigest(*record) || !discoveryInputsMatch(record.Inputs) {
			continue
		}
		valid := true
		for name, pin := range record.Pins {
			if validateRepositoryPin(name, pin) != nil {
				valid = false
				break
			}
		}
		for path, digest := range discovery.inputs {
			if record.Inputs[path] != digest {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		for name, pin := range record.Pins {
			session.pins[name] = pin
		}
		for path, digest := range record.Inputs {
			discovery.inputs[path] = digest
		}
		return
	}
}

// Resolution has succeeded and workers have stopped. Recheck inputs before
// publishing; a source edited during analysis must not bless an old selection.
func (session *dependencySession) storeDiscoveryCache() error {
	discovery := session.discovery
	if discovery == nil || discovery.disabled || !discoveryInputsMatch(discovery.inputs) {
		return nil
	}
	record := dependencyDiscoveryRecord{Version: 1, Key: discovery.key, Pins: session.pins, Inputs: discovery.inputs}
	record.Result = discoveryRecordDigest(record)
	selection := session.layout.buildKey
	if discovery.kind == "fetch-parse" {
		selection = session.layout.analysisKey
	}
	for _, path := range discovery.anchors {
		parse, ok, err := readParseCacheRecord(path)
		if err != nil {
			return err
		}
		if !ok || parse.Version != artifactCacheVersion || parse.Kind != discovery.kind || parse.Selection != selection {
			continue
		}
		result, err := parseResultFingerprint(parse)
		if err != nil || result != parse.Result {
			continue
		}
		parse.Discovery = &record
		parse.Result, err = parseResultFingerprint(parse)
		if err != nil {
			return err
		}
		contents, err := json.Marshal(parse)
		if err != nil {
			return err
		}
		return writeCacheRecord(path, append(contents, '\n'))
	}
	return nil
}
