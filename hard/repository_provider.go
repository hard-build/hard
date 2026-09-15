package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type repositoryReplacement struct {
	Source string `yaml:"source"`
	Ref    string `yaml:"ref"`
}

type repositoryConfiguration struct {
	Proxy struct {
		URL      string `yaml:"url"`
		Fallback bool   `yaml:"fallback"`
	} `yaml:"proxy"`
	Replace map[string]repositoryReplacement `yaml:"replace"`
	Auth    map[string]struct {
		TokenEnv string `yaml:"token_env"`
	} `yaml:"auth"`
}

var repositoryPartPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var repositoryCommitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var repositoryChecksumPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var credentialEnvironmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validRepositorySource(source string) bool {
	parts := strings.Split(source, "/")
	if len(parts) < 3 || !strings.Contains(parts[0], ".") {
		return false
	}
	for _, part := range parts {
		if part == "." || part == ".." || !repositoryPartPattern.MatchString(part) {
			return false
		}
	}
	return true
}

func validLogicalRepository(name string) bool {
	return validRepositorySource(name) && strings.HasPrefix(name, "github.com/") && len(strings.Split(name, "/")) == 3
}

func validateRepositoryPin(name string, pin repositoryPin) error {
	if !validLogicalRepository(name) || !validRepositorySource(pin.Source) || !validRepositoryRef(pin.Ref) ||
		!repositoryCommitPattern.MatchString(pin.Commit) || !repositoryChecksumPattern.MatchString(pin.Checksum) {
		return fmt.Errorf("invalid repository record %q; require source, ref, full commit and sha256 checksum", name)
	}
	return nil
}

func validRepositoryRef(ref string) bool {
	return ref != "" && !strings.ContainsAny(ref, "\x00\r\n") && !strings.HasPrefix(ref, "-")
}

func loadRepositoryConfiguration() (repositoryConfiguration, error) {
	var configuration repositoryConfiguration
	if filename := os.Getenv("HARD_CONFIG"); filename != "" {
		contents, err := readRegularProjectFile(filename)
		if err != nil {
			return configuration, fmt.Errorf("HARD_CONFIG: %w", err)
		}
		var document yaml.Node
		if err := decodeConfigurationYAML(contents, &configuration, &document); err != nil {
			return configuration, fmt.Errorf("HARD_CONFIG: %w", err)
		}
	}
	if proxy, ok := os.LookupEnv("HARD_PROXY"); ok {
		configuration.Proxy.URL = proxy
	}
	if configuration.Proxy.URL != "" {
		parsed, err := url.Parse(configuration.Proxy.URL)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !safeRepositoryURL(parsed) {
			return configuration, errors.New("dependency proxy must be an HTTPS URL without credentials, query or fragment (HTTP is allowed only on loopback)")
		}
		configuration.Proxy.URL = strings.TrimRight(configuration.Proxy.URL, "/")
	}
	for name, replacement := range configuration.Replace {
		if !validLogicalRepository(name) || !validRepositorySource(replacement.Source) || replacement.Ref != "" && !validRepositoryRef(replacement.Ref) {
			return configuration, fmt.Errorf("invalid replacement for %q", name)
		}
	}
	for host, auth := range configuration.Auth {
		parsed, err := url.Parse("https://" + host)
		if err != nil || parsed.Host != host || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || !credentialEnvironmentPattern.MatchString(auth.TokenEnv) || !strings.HasPrefix(auth.TokenEnv, "HARD_AUTH_") {
			return configuration, errors.New("auth requires an exact host[:port] and a HARD_AUTH_* token_env variable name")
		}
	}
	return configuration, nil
}

func safeRepositoryURL(address *url.URL) bool {
	if address.Scheme == "https" {
		return true
	}
	return address.Scheme == "http" && net.ParseIP(address.Hostname()).IsLoopback()
}

type repositoryProvider struct {
	configuration repositoryConfiguration
	client        *http.Client
	githubURL     string
}

func newRepositoryProvider(configuration repositoryConfiguration) *repositoryProvider {
	return &repositoryProvider{configuration: configuration, client: &http.Client{Timeout: 5 * time.Minute}, githubURL: githubAPIBaseURL}
}

type repositoryHTTPError struct{ status int }

func (err *repositoryHTTPError) Error() string {
	return fmt.Sprintf("dependency server returned HTTP %d", err.status)
}

func (provider *repositoryProvider) request(address string, mirror bool) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, errors.New("invalid dependency request URL")
	}
	request.Header.Set("User-Agent", "hard")
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	if err := provider.authorize(request); err != nil {
		return nil, err
	}
	client := *provider.client
	client.CheckRedirect = func(next *http.Request, previous []*http.Request) error {
		if len(previous) >= 10 || !safeRepositoryURL(next.URL) ||
			request.URL.Scheme == "https" && next.URL.Scheme != "https" ||
			mirror && (next.URL.Host != request.URL.Host || next.URL.Scheme != request.URL.Scheme) {
			return errors.New("dependency redirect is not allowed")
		}
		next.Header.Del("Authorization")
		return provider.authorize(next)
	}
	response, err := client.Do(request)
	if err != nil {
		// Transport errors can contain signed redirect URLs or credential material.
		return nil, errors.New("dependency request failed (network, TLS, timeout or disallowed redirect)")
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, &repositoryHTTPError{status: response.StatusCode}
	}
	return response, nil
}

func (provider *repositoryProvider) authorize(request *http.Request) error {
	if auth, ok := provider.configuration.Auth[request.URL.Host]; ok {
		token := os.Getenv(auth.TokenEnv)
		if token == "" || strings.ContainsAny(token, "\r\n") {
			return fmt.Errorf("credential environment variable %s is missing or invalid", auth.TokenEnv)
		}
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

func (provider *repositoryProvider) readJSON(address string, mirror bool, value any) error {
	response, err := provider.request(address, mirror)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(value); err != nil {
		return errors.New("invalid dependency server JSON response")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("invalid trailing dependency server JSON response")
	}
	return nil
}

func (provider *repositoryProvider) mayFallback(err error) bool {
	var status *repositoryHTTPError
	return provider.configuration.Proxy.Fallback && errors.As(err, &status) &&
		(status.status == 404 || status.status == 502 || status.status == 503 || status.status == 504)
}

func (provider *repositoryProvider) resolve(source, ref string) (repositoryPin, error) {
	pin := repositoryPin{Source: source, Ref: ref}
	if proxy := provider.configuration.Proxy.URL; proxy != "" {
		var result struct {
			Commit string `json:"commit"`
			Ref    string `json:"ref"`
		}
		query := url.Values{"source": {source}, "ref": {ref}}
		err := provider.readJSON(proxy+"/v1/resolve?"+query.Encode(), true, &result)
		if err == nil {
			if pin.Ref == "" {
				pin.Ref = result.Ref
			}
			pin.Commit = result.Commit
			if !repositoryCommitPattern.MatchString(pin.Commit) || !validRepositoryRef(pin.Ref) {
				return pin, errors.New("proxy returned an invalid commit or default ref")
			}
			return pin, nil
		}
		if !provider.mayFallback(err) {
			return pin, err
		}
	}
	if !validLogicalRepository(source) {
		return pin, fmt.Errorf("source %s requires a dependency proxy", source)
	}
	base := provider.githubURL + "/repos/" + strings.TrimPrefix(source, "github.com/")
	if pin.Ref == "" {
		var result struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := provider.readJSON(base, false, &result); err != nil {
			return pin, err
		}
		pin.Ref = result.DefaultBranch
		if !validRepositoryRef(pin.Ref) {
			return pin, errors.New("GitHub returned an invalid default branch")
		}
	}
	var result struct {
		SHA string `json:"sha"`
	}
	if err := provider.readJSON(base+"/commits/"+url.PathEscape(pin.Ref), false, &result); err != nil {
		return pin, err
	}
	if !repositoryCommitPattern.MatchString(result.SHA) {
		return pin, errors.New("GitHub returned an invalid commit")
	}
	pin.Commit = result.SHA
	return pin, nil
}

func (provider *repositoryProvider) snapshot(pin repositoryPin) (io.ReadCloser, error) {
	if proxy := provider.configuration.Proxy.URL; proxy != "" {
		query := url.Values{"source": {pin.Source}, "commit": {pin.Commit}}
		response, err := provider.request(proxy+"/v1/snapshot?"+query.Encode(), true)
		if err == nil {
			return response.Body, nil
		}
		if !provider.mayFallback(err) {
			return nil, err
		}
	}
	if !validLogicalRepository(pin.Source) {
		return nil, fmt.Errorf("source %s requires a dependency proxy", pin.Source)
	}
	response, err := provider.request(provider.githubURL+"/repos/"+strings.TrimPrefix(pin.Source, "github.com/")+"/tarball/"+pin.Commit, false)
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}
