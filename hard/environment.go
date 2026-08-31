package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const unavailableDiagnostic = "unavailable"

const (
	environmentReset  = "\x1b[0m"
	environmentBold   = "\x1b[1m"
	environmentDim    = "\x1b[2m"
	environmentGreen  = "\x1b[32m"
	environmentYellow = "\x1b[33m"
	environmentCyan   = "\x1b[36m"

	environmentTitle      = "HARD BUILD ENVIRONMENT"
	environmentRule       = "────────────────────────────────────────────────────────────"
	environmentLabelWidth = 20
)

type environmentReport struct {
	version            string
	executable         string
	runtimeRoot        string
	environment        string
	root               string
	operatingSystem    string
	kernel             string
	architecture       string
	cpu                string
	logicalCPUs        int
	libc               string
	compilerCommand    string
	compilerExecutable string
	compilerVersion    string
	compilerTarget     string
	executableSuffix   string
	executableRunner   string
	cflags             []string
	ldflags            []string
	entrypoints        []string
	libclang           string
	resourceDirectory  string
}

type environmentField struct {
	label string
	value string
}

type environmentStyle struct {
	noColor bool
}

func writeEnvironmentReport(
	config configuration,
	noColor bool,
	output io.Writer,
) error {
	report := collectEnvironmentReport(config)
	if _, err := io.WriteString(output, renderEnvironmentReport(report, noColor)); err != nil {
		return fmt.Errorf("write environment report: %w", err)
	}
	return nil
}

func collectEnvironmentReport(config configuration) environmentReport {
	compilerPath, compilerPathError := exec.LookPath(config.cc)
	compilerVersion := unavailableDiagnostic
	compilerTarget := unavailableDiagnostic
	if compilerPathError == nil {
		if absolutePath, err := filepath.Abs(compilerPath); err == nil {
			compilerPath = absolutePath
		}
		compilerVersion = diagnosticValue(diagnosticCommandFirstLine(compilerPath, "--version"))
		compilerTarget = diagnosticValue(diagnosticCommandFirstLine(compilerPath, "-dumpmachine"))
	} else {
		compilerPath = unavailableDiagnostic
	}

	resourceDirectory := unavailableDiagnostic
	if path, err := clangResourceDirectory(config.runtimeRoot); err == nil {
		resourceDirectory = path
		if resourceDirectory == "" {
			resourceDirectory = "system default"
		}
	}

	return environmentReport{
		version:            hardVersion(),
		executable:         resolvedExecutablePath(),
		runtimeRoot:        config.runtimeRoot,
		environment:        config.env,
		root:               config.root,
		operatingSystem:    diagnosticValue(readOSReleaseName("/etc/os-release")),
		kernel:             diagnosticValue(diagnosticCommandFirstLine("uname", "-sr")),
		architecture:       diagnosticValue(diagnosticCommandFirstLine("uname", "-m")),
		cpu:                diagnosticValue(readCPUModel("/proc/cpuinfo")),
		logicalCPUs:        runtime.NumCPU(),
		libc:               diagnosticValue(libcVersion()),
		compilerCommand:    quoteShellArgument(config.cc),
		compilerExecutable: compilerPath,
		compilerVersion:    compilerVersion,
		compilerTarget:     compilerTarget,
		executableSuffix:   displayExecutableSuffix(config.executableSuffix),
		executableRunner:   displayExecutableRunner(config.executableRunner),
		cflags:             append([]string(nil), config.cflags...),
		ldflags:            append([]string(nil), config.ldflags...),
		entrypoints:        append([]string(nil), config.entrypoints...),
		libclang:           clangVersion(),
		resourceDirectory:  resourceDirectory,
	}
}

func renderEnvironmentReport(report environmentReport, noColor bool) string {
	style := environmentStyle{noColor: noColor}
	var output strings.Builder
	fmt.Fprintf(
		&output,
		"%s\n%s\n%s\n",
		style.color(environmentDim+environmentCyan, environmentRule),
		style.color(environmentBold+environmentCyan, environmentTitle),
		style.color(environmentDim+environmentCyan, environmentRule),
	)
	writeEnvironmentSection(&output, style, "RUNTIME", []environmentField{
		{label: "Version", value: report.version},
		{label: "Executable", value: report.executable},
		{label: "Runtime root", value: report.runtimeRoot},
		{label: "Environment", value: report.environment},
		{label: "Data root", value: report.root},
	})
	writeEnvironmentSection(&output, style, "SYSTEM", []environmentField{
		{label: "Operating system", value: report.operatingSystem},
		{label: "Kernel", value: report.kernel},
		{label: "Architecture", value: report.architecture},
		{label: "CPU", value: report.cpu},
		{label: "Logical CPUs", value: strconv.Itoa(report.logicalCPUs)},
		{label: "libc", value: report.libc},
	})
	writeEnvironmentSection(&output, style, "COMPILER", []environmentField{
		{label: "Command", value: report.compilerCommand},
		{label: "Executable", value: report.compilerExecutable},
		{label: "Version", value: report.compilerVersion},
		{label: "Target", value: report.compilerTarget},
		{label: "Binary suffix", value: report.executableSuffix},
		{label: "Runner", value: report.executableRunner},
	})
	writeEnvironmentListSection(&output, style, "BUILD CONFIGURATION", []environmentField{
		{label: "CFLAGS", value: renderShellArgumentLines(report.cflags)},
		{label: "LDFLAGS", value: renderShellArgumentLines(report.ldflags)},
		{label: "Entry points", value: renderShellArgumentLines(report.entrypoints)},
	})
	writeEnvironmentSection(&output, style, "PARSER", []environmentField{
		{label: "libclang", value: report.libclang},
		{label: "Resource directory", value: report.resourceDirectory},
	})
	return output.String()
}

func writeEnvironmentSection(
	output *strings.Builder,
	style environmentStyle,
	title string,
	fields []environmentField,
) {
	fmt.Fprintf(output, "\n%s\n", style.color(environmentBold+environmentGreen, title))
	for _, field := range fields {
		label := fmt.Sprintf("  %-*s", environmentLabelWidth, field.label)
		fmt.Fprintf(output, "%s%s\n", style.color(environmentCyan, label), style.value(field.value))
	}
}

func writeEnvironmentListSection(
	output *strings.Builder,
	style environmentStyle,
	title string,
	fields []environmentField,
) {
	fmt.Fprintf(output, "\n%s\n", style.color(environmentBold+environmentGreen, title))
	for index, field := range fields {
		if index != 0 {
			output.WriteString("\n")
		}
		fmt.Fprintf(output, "%s\n", style.color(environmentCyan, "  "+field.label))
		for _, value := range strings.Split(field.value, "\n") {
			fmt.Fprintf(output, "    %s\n", style.value(value))
		}
	}
}

func (style environmentStyle) color(color, value string) string {
	if style.noColor {
		return value
	}
	return color + value + environmentReset
}

func (style environmentStyle) value(value string) string {
	switch value {
	case unavailableDiagnostic, "none", "direct", "system default":
		return style.color(environmentYellow, value)
	default:
		return value
	}
}

func resolvedExecutablePath() string {
	executable, err := os.Executable()
	if err != nil {
		return unavailableDiagnostic
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	if absolute, err := filepath.Abs(executable); err == nil {
		executable = absolute
	}
	return executable
}

func readOSReleaseName(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[name] = parseOSReleaseValue(value)
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if values["PRETTY_NAME"] != "" {
		return values["PRETTY_NAME"], nil
	}
	name := values["NAME"]
	if values["VERSION"] != "" {
		name = strings.TrimSpace(name + " " + values["VERSION"])
	}
	if name == "" {
		return "", errors.New("operating system name is absent")
	}
	return name, nil
}

func parseOSReleaseValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		if parsed, err := strconv.Unquote(value); err == nil {
			return parsed
		}
	}
	return value
}

func readCPUModel(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	wanted := []string{"model name", "hardware", "processor"}
	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name, value, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.TrimSpace(value)
		if value != "" && values[name] == "" {
			values[name] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	for _, name := range wanted {
		if values[name] != "" {
			return values[name], nil
		}
	}
	return "", errors.New("CPU model is absent")
}

func libcVersion() (string, error) {
	if version, err := diagnosticCommandFirstLine("getconf", "GNU_LIBC_VERSION"); err == nil {
		return version, nil
	}
	return diagnosticCommandFirstLine("ldd", "--version")
}

func diagnosticCommandFirstLine(name string, arguments ...string) (string, error) {
	output, err := exec.Command(name, arguments...).CombinedOutput()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", errors.New("command returned no output")
}

func diagnosticValue(value string, err error) string {
	if err != nil || value == "" {
		return unavailableDiagnostic
	}
	return value
}

func displayExecutableSuffix(suffix string) string {
	if suffix == "" {
		return "none"
	}
	return suffix
}

func displayExecutableRunner(runner string) string {
	if runner == "" {
		return "direct"
	}
	return quoteShellArgument(runner)
}

func renderShellArgumentLines(arguments []string) string {
	if len(arguments) == 0 {
		return "none"
	}
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = quoteShellArgument(argument)
	}
	return strings.Join(quoted, "\n")
}
