package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	githubAPIBaseURL = "https://api.github.com"
	githubAPIVersion = "2022-11-28"
)

type githubRepository struct {
	owner string
	name  string
}

type wellKnownGitHubRepository struct {
	include    string
	repository githubRepository
}

var wellKnownGitHubRepositories = []wellKnownGitHubRepository{
	{
		include: "hard",
		repository: githubRepository{
			owner: "hard-build",
			name:  "library",
		},
	},
	{
		include: "recipe",
		repository: githubRepository{
			owner: "hard-build",
			name:  "recipe",
		},
	},
}

func (repository githubRepository) key() string {
	return repository.owner + "/" + repository.name
}

type githubSnapshotState struct {
	done chan struct{}
	err  error
}

type githubSnapshotResolver struct {
	root       string
	client     *http.Client
	apiBaseURL string
	progress   *progressBar

	mutex        sync.Mutex
	repositories map[string]*githubSnapshotState
	downloads    []string
}

func newGitHubSnapshotResolver(root string, progress *progressBar) *githubSnapshotResolver {
	return newGitHubSnapshotResolverWithClient(
		root,
		&http.Client{Timeout: 5 * time.Minute},
		githubAPIBaseURL,
		progress,
	)
}

func newGitHubSnapshotResolverWithClient(
	root string,
	client *http.Client,
	apiBaseURL string,
	progress *progressBar,
) *githubSnapshotResolver {
	return &githubSnapshotResolver{
		root:         root,
		client:       client,
		apiBaseURL:   strings.TrimRight(apiBaseURL, "/"),
		progress:     progress,
		repositories: make(map[string]*githubSnapshotState),
	}
}

func (resolver *githubSnapshotResolver) downloadProgressEntries() []string {
	resolver.mutex.Lock()
	defer resolver.mutex.Unlock()
	return append([]string(nil), resolver.downloads...)
}

func (resolver *githubSnapshotResolver) ensure(repository githubRepository) error {
	destination, err := githubRepositoryDirectory(
		resolver.root,
		repository.owner,
		repository.name,
	)
	if err != nil {
		return err
	}
	exists, err := existingGitHubRepository(destination)
	if err != nil {
		return err
	}
	if exists {
		return ensureWellKnownGitHubRepositoryAliases(
			resolver.root,
			repository,
			destination,
		)
	}

	resolver.mutex.Lock()
	if state, ok := resolver.repositories[repository.key()]; ok {
		resolver.mutex.Unlock()
		<-state.done
		return state.err
	}
	state := &githubSnapshotState{done: make(chan struct{})}
	resolver.repositories[repository.key()] = state
	resolver.mutex.Unlock()

	state.err = resolver.download(repository, destination)
	if state.err == nil {
		state.err = ensureWellKnownGitHubRepositoryAliases(
			resolver.root,
			repository,
			destination,
		)
	}
	close(state.done)
	return state.err
}

func (resolver *githubSnapshotResolver) download(
	repository githubRepository,
	destination string,
) error {
	exists, err := existingGitHubRepository(destination)
	if err != nil || exists {
		return err
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create GitHub repository parent %s: %w", parent, err)
	}
	temporary, err := os.MkdirTemp(parent, "."+repository.name+".*")
	if err != nil {
		return fmt.Errorf("create GitHub snapshot directory for %s: %w", repository.key(), err)
	}
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.RemoveAll(temporary)
		}
	}()

	requestURL := resolver.apiBaseURL + "/repos/" +
		url.PathEscape(repository.owner) + "/" +
		url.PathEscape(repository.name) + "/tarball"
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("create GitHub snapshot request for %s: %w", repository.key(), err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	request.Header.Set("User-Agent", "hard/1.0")
	resolver.mutex.Lock()
	resolver.downloads = append(
		resolver.downloads,
		"Downloading github.com/"+repository.key(),
	)
	resolver.mutex.Unlock()
	if resolver.progress != nil {
		resolver.progress.updateStep("Downloading github.com/" + repository.key())
	}
	response, err := resolver.client.Do(request)
	if err != nil {
		return fmt.Errorf("download GitHub snapshot %s: %w", repository.key(), err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		message := strings.TrimSpace(string(detail))
		if message == "" {
			return fmt.Errorf(
				"download GitHub snapshot %s: %s",
				repository.key(),
				response.Status,
			)
		}
		return fmt.Errorf(
			"download GitHub snapshot %s: %s: %s",
			repository.key(),
			response.Status,
			message,
		)
	}
	if err := extractGitHubSnapshot(response.Body, temporary); err != nil {
		return fmt.Errorf("extract GitHub snapshot %s: %w", repository.key(), err)
	}

	exists, err = existingGitHubRepository(destination)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err := os.Rename(temporary, destination); err != nil {
		return fmt.Errorf("install GitHub snapshot %s: %w", repository.key(), err)
	}
	removeTemporary = false
	return nil
}

func existingGitHubRepository(directory string) (bool, error) {
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect GitHub repository cache %s: %w", directory, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("GitHub repository cache is not a directory: %s", directory)
	}
	return true, nil
}

func githubRepositoryDirectory(root, owner, repository string) (string, error) {
	if !validGitHubPathSegment(owner) || !validGitHubPathSegment(repository) {
		return "", fmt.Errorf("invalid GitHub repository path: %s/%s", owner, repository)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("make HARD_ROOT absolute: %w", err)
	}
	repositoryRoot := filepath.Join(absoluteRoot, "source", "github.com")
	directory := filepath.Join(repositoryRoot, owner, repository)
	if !pathWithin(repositoryRoot, directory) {
		return "", fmt.Errorf("GitHub repository path escapes source directory: %s/%s", owner, repository)
	}
	return directory, nil
}

func ensureWellKnownGitHubRepositoryAliases(
	root string,
	repository githubRepository,
	destination string,
) error {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("make HARD_ROOT absolute: %w", err)
	}
	sourceRoot := filepath.Join(absoluteRoot, "source")
	for _, wellKnown := range wellKnownGitHubRepositories {
		if wellKnown.repository != repository {
			continue
		}
		alias := filepath.Join(sourceRoot, wellKnown.include)
		if !pathWithin(sourceRoot, alias) {
			return fmt.Errorf(
				"well-known repository alias escapes source directory: %s",
				wellKnown.include,
			)
		}
		if err := ensureWellKnownGitHubRepositoryAlias(alias, destination); err != nil {
			return err
		}
	}
	return nil
}

func ensureWellKnownGitHubRepositoryAlias(alias, destination string) error {
	target, err := filepath.Rel(filepath.Dir(alias), destination)
	if err != nil {
		return fmt.Errorf("resolve well-known repository alias %s: %w", alias, err)
	}
	if err := os.Symlink(target, alias); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create well-known repository alias %s: %w", alias, err)
	}

	info, err := os.Lstat(alias)
	if err != nil {
		return fmt.Errorf("inspect well-known repository alias %s: %w", alias, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("well-known repository alias is not a symbolic link: %s", alias)
	}
	resolvedAlias, err := filepath.EvalSymlinks(alias)
	if err != nil {
		return fmt.Errorf("resolve well-known repository alias %s: %w", alias, err)
	}
	resolvedDestination, err := filepath.EvalSymlinks(destination)
	if err != nil {
		return fmt.Errorf("resolve well-known repository destination %s: %w", destination, err)
	}
	if filepath.Clean(resolvedAlias) != filepath.Clean(resolvedDestination) {
		return fmt.Errorf(
			"well-known repository alias points to an unexpected path: %s",
			alias,
		)
	}
	return nil
}

func validGitHubPathSegment(segment string) bool {
	return segment != "" &&
		segment != "." &&
		segment != ".." &&
		filepath.Base(segment) == segment &&
		filepath.Clean(segment) == segment
}

func githubRepositoriesFromDependencies(dependencies []string) []githubRepository {
	seen := make(map[string]struct{})
	repositories := make([]githubRepository, 0)
	for _, dependency := range dependencies {
		dependency = filepath.ToSlash(dependency)
		repository, ok := githubRepositoryFromDependency(dependency)
		if !ok {
			continue
		}
		if _, ok := seen[repository.key()]; ok {
			continue
		}
		seen[repository.key()] = struct{}{}
		repositories = append(repositories, repository)
	}
	return repositories
}

func githubRepositoryFromDependency(dependency string) (githubRepository, bool) {
	if strings.HasPrefix(dependency, "github.com/") {
		parts := strings.Split(dependency, "/")
		if len(parts) < 4 ||
			!validGitHubPathSegment(parts[1]) ||
			!validGitHubPathSegment(parts[2]) {
			return githubRepository{}, false
		}
		return githubRepository{owner: parts[1], name: parts[2]}, true
	}
	for _, wellKnown := range wellKnownGitHubRepositories {
		prefix := wellKnown.include + "/"
		if strings.HasPrefix(dependency, prefix) && len(dependency) > len(prefix) {
			return wellKnown.repository, true
		}
	}
	return githubRepository{}, false
}

func extractGitHubSnapshot(input io.Reader, destination string) error {
	compressed, err := gzip.NewReader(input)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer compressed.Close()

	archive := tar.NewReader(compressed)
	archiveRoot := ""
	extracted := false
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}
		if header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		relative, rootOnly, err := githubArchivePath(header.Name, &archiveRoot)
		if err != nil {
			return err
		}
		if rootOnly {
			if header.Typeflag != tar.TypeDir {
				return fmt.Errorf("archive root is not a directory: %s", header.Name)
			}
			continue
		}
		if relative == ".git" || strings.HasPrefix(relative, ".git/") {
			return fmt.Errorf("archive contains forbidden .git path: %s", header.Name)
		}
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if !pathWithin(destination, target) {
			return fmt.Errorf("archive entry escapes destination: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := createGitHubArchiveDirectory(destination, target); err != nil {
				return fmt.Errorf("extract directory %s: %w", header.Name, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := writeGitHubArchiveFile(destination, target, header, archive); err != nil {
				return fmt.Errorf("extract file %s: %w", header.Name, err)
			}
			extracted = true
		case tar.TypeSymlink:
			if err := writeGitHubArchiveSymlink(
				destination,
				target,
				relative,
				header.Linkname,
			); err != nil {
				return fmt.Errorf("extract symlink %s: %w", header.Name, err)
			}
			extracted = true
		default:
			return fmt.Errorf(
				"unsupported archive entry type %d: %s",
				header.Typeflag,
				header.Name,
			)
		}
	}
	if archiveRoot == "" || !extracted {
		return errors.New("GitHub snapshot archive is empty")
	}
	return nil
}

func githubArchivePath(name string, archiveRoot *string) (string, bool, error) {
	if name == "" || path.IsAbs(name) {
		return "", false, fmt.Errorf("invalid archive entry path: %s", name)
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false, fmt.Errorf("archive entry escapes root: %s", name)
	}
	root, relative, found := strings.Cut(clean, "/")
	if root == "" || root == "." || root == ".." {
		return "", false, fmt.Errorf("invalid archive root path: %s", name)
	}
	if *archiveRoot == "" {
		*archiveRoot = root
	} else if *archiveRoot != root {
		return "", false, fmt.Errorf(
			"archive has multiple root directories: %s and %s",
			*archiveRoot,
			root,
		)
	}
	if !found || relative == "" {
		return "", true, nil
	}
	return relative, false, nil
}

func createGitHubArchiveDirectory(root, target string) error {
	target, err := githubArchiveTarget(root, target)
	if err != nil {
		return err
	}
	info, err := os.Lstat(target)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("archive directory target already exists and is not a directory: %s", target)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Mkdir(target, 0o755)
}

func writeGitHubArchiveFile(
	root string,
	target string,
	header *tar.Header,
	input io.Reader,
) error {
	target, err := githubArchiveTarget(root, target)
	if err != nil {
		return err
	}
	mode := os.FileMode(header.Mode).Perm()
	if mode == 0 {
		mode = 0o644
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, input); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeGitHubArchiveSymlink(
	root string,
	target string,
	relative string,
	linkname string,
) error {
	if linkname == "" || path.IsAbs(linkname) || filepath.IsAbs(filepath.FromSlash(linkname)) {
		return fmt.Errorf("invalid symlink target: %s", linkname)
	}
	resolved := filepath.Clean(filepath.Join(
		filepath.Dir(filepath.FromSlash(relative)),
		filepath.FromSlash(linkname),
	))
	if resolved == ".." || strings.HasPrefix(resolved, ".."+string(filepath.Separator)) {
		return fmt.Errorf("symlink target escapes archive: %s", linkname)
	}
	target, err := githubArchiveTarget(root, target)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return fmt.Errorf("archive symlink target already exists: %s", target)
		}
		return err
	}
	return os.Symlink(filepath.FromSlash(linkname), target)
}

func githubArchiveTarget(root, target string) (string, error) {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !pathWithin(root, resolvedParent) {
		return "", fmt.Errorf("archive target parent escapes destination: %s", target)
	}
	return filepath.Join(resolvedParent, filepath.Base(target)), nil
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
