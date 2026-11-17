package conf

// Options configures optional strict parsing behavior.
// Zero-value options preserve the legacy Parse and ParseFile behavior.
type Options struct {
 // IncludeRoot restricts file reads, including the initial file, to this directory.
 // Symlinks are resolved before checking containment.
 IncludeRoot string
 // EnvAsString treats environment variable references as literal strings.
 // The default retains legacy typed configuration parsing.
 EnvAsString bool
 // RejectDuplicateKeys returns an error when a map key is assigned twice.
 RejectDuplicateKeys bool
}

// ParseWithOptions parses configuration text with explicit options.
func ParseWithOptions(data string, opts Options) (map[string]any, error) {
 p, err := parseDataWithOptions(data, "", false, make(map[string]bool), opts)
 if err != nil { return nil, err }
 return p.mapping, nil
}

// ParseFileWithOptions parses a configuration file with explicit options.
func ParseFileWithOptions(path string, opts Options) (map[string]any, error) {
 return parseFileWithOptions(path, false, make(map[string]bool), opts)
}

// ParseWithChecksAndOptions returns token-wrapped values with options applied.
func ParseWithChecksAndOptions(data string, opts Options) (map[string]any, error) {
 p, err := parseDataWithOptions(data, "", true, make(map[string]bool), opts)
 if err != nil { return nil, err }
 return p.mapping, nil
}

// ParseFileWithChecksAndOptions returns token-wrapped values with options applied.
func ParseFileWithChecksAndOptions(path string, opts Options) (map[string]any, error) {
 return parseFileWithOptions(path, true, make(map[string]bool), opts)
}
