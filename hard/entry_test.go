package main

import (
	"strings"
	"testing"
)

func TestExtractEntryPoint(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		entrypoints []string
		want        string
	}{
		{
			name:        "main definition",
			source:      "int main(int argc, char **argv) { return argc + (argv != nullptr); }\n",
			entrypoints: []string{"main", "_start"},
			want:        "main",
		},
		{
			name:        "configured custom definition",
			source:      "void application_start() {}\n",
			entrypoints: []string{"application_start"},
			want:        "application_start",
		},
		{
			name:        "extern C definition",
			source:      "extern \"C\" { void _start() {} }\n",
			entrypoints: []string{"_start"},
			want:        "_start",
		},
		{
			name:        "declaration only",
			source:      "int main();\n",
			entrypoints: []string{"main"},
		},
		{
			name:        "unconfigured function",
			source:      "int main() { return 0; }\n",
			entrypoints: []string{"_start"},
		},
		{
			name:        "namespace function",
			source:      "namespace application { void start() {} }\n",
			entrypoints: []string{"start"},
		},
		{
			name:        "qualified namespace definition",
			source:      "namespace application { void start(); } void application::start() {}\n",
			entrypoints: []string{"start"},
		},
		{
			name:        "class method",
			source:      "struct application { static void start() {} };\n",
			entrypoints: []string{"start"},
		},
		{
			name:        "preprocessor branch",
			source:      "#if ENABLED\nint main() { return 0; }\n#endif\n",
			entrypoints: []string{"main"},
		},
		{
			name:        "macro definition",
			source:      "#define ENTRY(name) int name() { return 0; }\nENTRY(main)\n",
			entrypoints: []string{"main"},
			want:        "main",
		},
		{
			name:        "empty configuration",
			source:      "int main() { return 0; }\n",
			entrypoints: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractEntryPoint([]byte(tt.source), tt.entrypoints)
			if err != nil {
				t.Fatalf("extractEntryPoint() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("extractEntryPoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractEntryPointRejectsMultipleConfiguredDefinitions(t *testing.T) {
	_, err := extractEntryPoint(
		[]byte("int main() { return 0; }\nextern \"C\" void _start() {}\n"),
		[]string{"main", "_start"},
	)
	if err == nil {
		t.Fatal("extractEntryPoint() error = nil")
	}
	for _, name := range []string{"main", "_start"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("extractEntryPoint() error = %q, want %q", err, name)
		}
	}
}

func TestExtractEntryPointDeduplicatesConditionalDefinitions(t *testing.T) {
	source := "#if FIRST\nint main() { return 1; }\n#else\nint main() { return 2; }\n#endif\n"
	got, err := extractEntryPoint([]byte(source), []string{"main"})
	if err != nil {
		t.Fatalf("extractEntryPoint() error = %v", err)
	}
	if got != "main" {
		t.Fatalf("extractEntryPoint() = %q, want main", got)
	}
}
