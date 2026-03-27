package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_js "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_py "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_rs "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tree_sitter_ts "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

type Symbol struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Line int    `json:"line"`
}

type FileResult struct {
	Hash    string   `json:"hash"`
	RelPath string   `json:"rel_path"`
	Lang    string   `json:"lang"`
	Symbols []Symbol `json:"symbols"`
}

// langDef bundles everything needed for one language.
type langDef struct {
	Extensions []string
	Language   func() unsafe.Pointer
	Query      string
}

var languages = map[string]langDef{
	"go": {
		Extensions: []string{".go"},
		Language:   tree_sitter_go.Language,
		Query: `
(function_declaration name: (identifier) @name (#set! kind "func"))
(method_declaration name: (field_identifier) @name (#set! kind "method"))
(type_spec name: (type_identifier) @name (#set! kind "type"))
`,
	},
	"python": {
		Extensions: []string{".py"},
		Language:   tree_sitter_py.Language,
		Query: `
(function_definition name: (identifier) @name (#set! kind "func"))
(class_definition name: (identifier) @name (#set! kind "class"))
`,
	},
	"javascript": {
		Extensions: []string{".js", ".jsx", ".mjs"},
		Language:   tree_sitter_js.Language,
		Query: `
(function_declaration name: (identifier) @name (#set! kind "func"))
(class_declaration name: (identifier) @name (#set! kind "class"))
(method_definition name: (property_identifier) @name (#set! kind "method"))
`,
	},
	"typescript": {
		Extensions: []string{".ts", ".tsx"},
		Language:   tree_sitter_ts.LanguageTypescript,
		Query: `
(function_declaration name: (identifier) @name (#set! kind "func"))
(class_declaration name: (identifier) @name (#set! kind "class"))
(method_definition name: (property_identifier) @name (#set! kind "method"))
(interface_declaration name: (type_identifier) @name (#set! kind "interface"))
(type_alias_declaration name: (type_identifier) @name (#set! kind "type"))
`,
	},
	"rust": {
		Extensions: []string{".rs"},
		Language:   tree_sitter_rs.Language,
		Query: `
(function_item name: (identifier) @name (#set! kind "func"))
(struct_item name: (type_identifier) @name (#set! kind "struct"))
(enum_item name: (type_identifier) @name (#set! kind "enum"))
(trait_item name: (type_identifier) @name (#set! kind "trait"))
(impl_item type: (type_identifier) @name (#set! kind "impl"))
`,
	},
}

// extToLang maps a file extension to a language name.
var extMap map[string]string

func init() {
	extMap = make(map[string]string)
	for name, def := range languages {
		for _, ext := range def.Extensions {
			extMap[ext] = name
		}
	}
}

// parseFile extracts symbols from source code using tree-sitter.
func parseFile(lang string, source []byte) ([]Symbol, error) {
	def, ok := languages[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}

	tsLang := tree_sitter.NewLanguage(unsafe.Pointer(def.Language()))

	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(tsLang); err != nil {
		return nil, err
	}

	tree := parser.Parse(source, nil)
	defer tree.Close()

	root := tree.RootNode()

	query, err := tree_sitter.NewQuery(tsLang, def.Query)
	if err != nil {
		return nil, fmt.Errorf("query compile: %w", err)
	}
	defer query.Close()

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	nameIdx, hasName := query.CaptureIndexForName("name")
	if !hasName {
		return nil, nil
	}

	var symbols []Symbol
	seen := make(map[string]bool)

	matches := cursor.Matches(query, root, source)
	for match := matches.Next(); match != nil; match = matches.Next() {
		// Get the kind from the pattern's #set! metadata
		patIdx := match.PatternIndex
		kind := "symbol"
		settings := query.PropertySettings(patIdx)
		for _, s := range settings {
			if s.Key == "kind" && s.Value != nil {
				kind = *s.Value
			}
		}

		nodes := match.NodesForCaptureIndex(nameIdx)
		for _, node := range nodes {
			name := node.Utf8Text(source)
			line := int(node.StartPosition().Row) + 1
			key := fmt.Sprintf("%s:%s:%d", kind, name, line)
			if seen[key] {
				continue
			}
			seen[key] = true
			symbols = append(symbols, Symbol{Kind: kind, Name: name, Line: line})
		}
	}

	return symbols, nil
}

func hashBytes(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func cacheDir(repoRoot string) string {
	h := sha256.Sum256([]byte(repoRoot))
	id := hex.EncodeToString(h[:8])
	return filepath.Join(os.TempDir(), "opc", "scan", id)
}

func loadCache(dir string) map[string]FileResult {
	data, err := os.ReadFile(filepath.Join(dir, "cache.json"))
	if err != nil {
		return make(map[string]FileResult)
	}
	var entries map[string]FileResult
	if err := json.Unmarshal(data, &entries); err != nil {
		return make(map[string]FileResult)
	}
	return entries
}

func saveCache(dir string, entries map[string]FileResult) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "cache.json"), data, 0644)
}

func gitTrackedFiles(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

// Repo scans a git repository with Tree-sitter and returns a markdown code map.
// Results are cached per-file by content hash so subsequent calls only re-parse changed files.
func Repo(repoRoot string) (string, error) {
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); os.IsNotExist(err) {
		return "", nil
	}

	files, err := gitTrackedFiles(repoRoot)
	if err != nil {
		return "", fmt.Errorf("listing files: %w", err)
	}

	dir := cacheDir(repoRoot)
	cache := loadCache(dir)
	newCache := make(map[string]FileResult)

	var results []FileResult
	parsed, cached, skipped := 0, 0, 0

	for _, relPath := range files {
		lang, ok := extMap[filepath.Ext(relPath)]
		if !ok {
			skipped++
			continue
		}

		content, err := os.ReadFile(filepath.Join(repoRoot, relPath))
		if err != nil {
			continue
		}

		hash := hashBytes(content)

		if entry, ok := cache[relPath]; ok && entry.Hash == hash {
			newCache[relPath] = entry
			if len(entry.Symbols) > 0 {
				results = append(results, entry)
			}
			cached++
			continue
		}

		symbols, err := parseFile(lang, content)
		if err != nil {
			continue
		}

		entry := FileResult{Hash: hash, RelPath: relPath, Lang: lang, Symbols: symbols}
		newCache[relPath] = entry
		if len(symbols) > 0 {
			results = append(results, entry)
		}
		parsed++
	}

	_ = saveCache(dir, newCache)

	if len(results) == 0 {
		return "", nil
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].RelPath < results[j].RelPath
	})

	var b strings.Builder
	b.WriteString("## Code Map\n")
	b.WriteString(fmt.Sprintf("\n_%d files parsed, %d cached, %d skipped_\n", parsed, cached, skipped))

	for _, r := range results {
		b.WriteString(fmt.Sprintf("\n### %s\n", r.RelPath))
		for _, s := range r.Symbols {
			b.WriteString(fmt.Sprintf("- %s **%s** (L%d)\n", s.Kind, s.Name, s.Line))
		}
	}

	return b.String(), nil
}
