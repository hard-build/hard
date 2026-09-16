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
	filename, key string
	read          bool
	mutex         sync.Mutex
	inputs        map[string]string
	disabled      bool
}

type dependencyDiscoveryRecord struct {
	Version int                      `json:"version"`
	Key     string                   `json:"key"`
	Pins    map[string]repositoryPin `json:"repositories"`
	Inputs  map[string]string        `json:"inputs"`
	Result  string                   `json:"result"`
}

func (session *dependencySession) prepareDiscoveryCache(configuration configuration, parsed arguments, sources []string) error {
	if session == nil || session.record {
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
	session.discovery = &dependencyDiscovery{
		filename: filepath.Join(owner, "dependencies.json"), key: repositoryDigest(context),
		read: !parsed.noCache, inputs: make(map[string]string),
	}
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
	contents, err := readRegularProjectFile(discovery.filename)
	if err != nil {
		return
	}
	var record dependencyDiscoveryRecord
	if json.Unmarshal(contents, &record) != nil || record.Version != 1 || record.Key != discovery.key ||
		record.Result != discoveryRecordDigest(record) || !discoveryInputsMatch(record.Inputs) {
		return
	}
	for name, pin := range record.Pins {
		if validateRepositoryPin(name, pin) != nil {
			return
		}
	}
	for path, digest := range discovery.inputs {
		if record.Inputs[path] != digest {
			return
		}
	}
	for name, pin := range record.Pins {
		session.pins[name] = pin
	}
	for path, digest := range record.Inputs {
		discovery.inputs[path] = digest
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
	contents, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return writeCacheRecord(discovery.filename, append(contents, '\n'))
}
