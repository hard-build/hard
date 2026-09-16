package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type repositoryRequirement struct {
	owner    string
	snapshot string
	pin      repositoryPin
}

func (requirement repositoryRequirement) origin() string {
	return requirement.owner + "@" + filepath.Base(requirement.snapshot) + "/" + projectFilename
}

func sameRepositoryContents(left, right repositoryPin) bool {
	return left.Source == right.Source && left.Commit == right.Commit && left.Checksum == right.Checksum
}

func repositoryRequirementConflict(name, leftOrigin string, left repositoryPin, rightOrigin string, right repositoryPin) error {
	return fmt.Errorf("repository pin conflict for %s: %s selects %s@%s (ref %q, %s); %s requires %s@%s (ref %q, %s)",
		name, leftOrigin, left.Source, left.Commit, left.Ref, left.Checksum,
		rightOrigin, right.Source, right.Commit, right.Ref, right.Checksum)
}

// Called with the session mutex held. Only the current immutable view can own
// an including file; an unrelated cached snapshot must not supply requirements.
func (session *dependencySession) fileRepository(filename string) (string, error) {
	if filename == "" {
		return "", nil
	}
	canonical, err := filepath.EvalSymlinks(filename)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(session.selected))
	for name := range session.selected {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		snapshot, err := filepath.EvalSymlinks(session.selected[name])
		if err != nil {
			return "", err
		}
		if pathWithin(snapshot, canonical) {
			return name, nil
		}
	}
	return "", nil
}

func (session *dependencySession) inheritedRequirement(name, parent string) (*repositoryRequirement, error) {
	owner, err := session.fileRepository(parent)
	if err != nil {
		return nil, err
	}
	if owner != "" && owner != name {
		snapshot := session.selected[owner]
		if snapshot != session.snapshots[owner] {
			return nil, errDependencySetChanged
		}
		manifest, loaded := session.manifests[snapshot]
		if !loaded {
			manifest, err = readProjectFile(filepath.Join(snapshot, projectFilename))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("dependency configuration for %s: %w", owner, err)
			}
			session.manifests[snapshot] = manifest
		}
		if manifest != nil {
			if pin, found := manifest.Repositories[name]; found {
				if session.requirements[name] == nil {
					session.requirements[name] = make(map[string]repositoryRequirement)
				}
				session.requirements[name][owner] = repositoryRequirement{owner: owner, snapshot: snapshot, pin: pin}
			}
		}
	}
	// Any recorded project choice, including an explicit update, takes precedence
	// over inherited pins and resolves disagreements between their owners.
	if _, recorded := session.project.Repositories[name]; recorded {
		return nil, nil
	}
	owners := make([]string, 0, len(session.requirements[name]))
	for owner := range session.requirements[name] {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	var selected *repositoryRequirement
	for _, owner := range owners {
		requirement := session.requirements[name][owner]
		if requirement.snapshot != session.snapshots[owner] {
			continue
		}
		// An explicit corporate replacement overrides upstream requirements, but
		// an identical source/ref rule does not discard the inherited checksum.
		if replacement, ok := session.provider.configuration.Replace[name]; ok &&
			(replacement.Source != requirement.pin.Source || replacement.Ref != "" && replacement.Ref != requirement.pin.Ref) {
			continue
		}
		if selected != nil && !sameRepositoryContents(selected.pin, requirement.pin) {
			return nil, repositoryRequirementConflict(name, selected.origin(), selected.pin, requirement.origin(), requirement.pin)
		}
		if selected == nil {
			selected = &requirement
		}
	}
	return selected, nil
}

// Unlike the legacy missing-include downloader, pin validation also visits
// already resolved edges. Otherwise a warm source view would hide conflicts.
func (resolver *githubSnapshotResolver) prepareInheritedIncludes(analysis clangAnalysis, workingDirectory string) error {
	if resolver == nil || resolver.session == nil {
		return nil
	}
	for _, include := range analysis.includes {
		if include.system {
			continue
		}
		repository, managed := githubRepositoryFromDependency(filepath.ToSlash(include.spelling))
		if include.target != "" {
			target, err := realAbsolutePath(include.target, workingDirectory)
			if err != nil {
				return err
			}
			resolver.session.mutex.Lock()
			name, err := resolver.session.fileRepository(target)
			resolver.session.mutex.Unlock()
			if err != nil {
				return err
			}
			if name == "" {
				continue
			}
			repository, err = libraryRecipeRepository(name)
			if err != nil {
				return err
			}
			managed = true
		}
		if !managed {
			continue
		}
		parent := include.source
		if parent != "" {
			var err error
			parent, err = realAbsolutePath(parent, workingDirectory)
			if err != nil {
				return err
			}
		}
		if err := resolver.ensure(repository, parent); err != nil {
			return err
		}
	}
	return nil
}
