// Package skills implements the Agent Skills open standard.
// See https://agentskills.io for the specification.
package skills

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/charlievieth/fastwalk"
	"gopkg.in/yaml.v3"
)

const (
	SkillFileName          = "SKILL.md"
	MaxNameLength          = 64
	MaxDescriptionLength   = 1024
	MaxCompatibilityLength = 500
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

var promptInjectionDetectors = []struct {
	Name    string
	Pattern *regexp.Regexp
}{
	{Name: "ignore_instructions", Pattern: regexp.MustCompile(`(?i)\b(ignore|disregard|bypass|override)\b[^\n\.]{0,120}\b(instruction|system prompt|developer message|policy|safety)\b`)},
	{Name: "prompt_exfiltration", Pattern: regexp.MustCompile(`(?i)\b(reveal|print|dump|show|expose|leak|exfiltrate)\b[^\n\.]{0,120}\b(system prompt|developer message|hidden instruction|secret|token|api key|credential)\b`)},
	{Name: "authority_impersonation", Pattern: regexp.MustCompile(`(?i)\b(you are now|from now on|act as|pretend to be)\b[^\n\.]{0,120}\b(system|developer|administrator|root)\b`)},
	{Name: "tool_bypass", Pattern: regexp.MustCompile(`(?i)\b(run|execute)\b[^\n\.]{0,120}\b(without permission|without approval|without confirmation|silently)\b`)},
	{Name: "data_exfiltration", Pattern: regexp.MustCompile(`(?i)\b(send|upload|post|exfiltrate)\b[^\n\.]{0,120}\b(secret|token|credential|password|cookie|env)\b`)},
}

// Skill represents a parsed SKILL.md file.
type Skill struct {
	Name          string            `yaml:"name" json:"name"`
	Description   string            `yaml:"description" json:"description"`
	License       string            `yaml:"license,omitempty" json:"license,omitempty"`
	Compatibility string            `yaml:"compatibility,omitempty" json:"compatibility,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Instructions  string            `yaml:"-" json:"instructions"`
	Path          string            `yaml:"-" json:"path"`
	SkillFilePath string            `yaml:"-" json:"skill_file_path"`
	Builtin       bool              `yaml:"-" json:"builtin"`
}

// Inspection captures parsed skill metadata and safety outcome without
// requiring full validation success.
type Inspection struct {
	Name               string
	Description        string
	Path               string
	SkillFilePath      string
	Blocked            bool
	BlockReasons       []string
	PermissionWarnings []string
}

// Validate checks if the skill meets spec requirements.
func (s *Skill) Validate() error {
	var errs []error

	if s.Name == "" {
		errs = append(errs, errors.New("name is required"))
	} else {
		if len(s.Name) > MaxNameLength {
			errs = append(errs, fmt.Errorf("name exceeds %d characters", MaxNameLength))
		}
		if !namePattern.MatchString(s.Name) {
			errs = append(errs, errors.New("name must be alphanumeric with hyphens, no leading/trailing/consecutive hyphens"))
		}
		if s.Path != "" && !strings.EqualFold(filepath.Base(s.Path), s.Name) {
			errs = append(errs, fmt.Errorf("name %q must match directory %q", s.Name, filepath.Base(s.Path)))
		}
	}

	if s.Description == "" {
		errs = append(errs, errors.New("description is required"))
	} else if len(s.Description) > MaxDescriptionLength {
		errs = append(errs, fmt.Errorf("description exceeds %d characters", MaxDescriptionLength))
	}

	if len(s.Compatibility) > MaxCompatibilityLength {
		errs = append(errs, fmt.Errorf("compatibility exceeds %d characters", MaxCompatibilityLength))
	}

	if detectorNames := s.PromptInjectionDetectors(); len(detectorNames) > 0 {
		errs = append(errs, fmt.Errorf("prompt-injection safeguard triggered: %s", strings.Join(detectorNames, ", ")))
	}

	return errors.Join(errs...)
}

// Parse parses a SKILL.md file from disk.
func Parse(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	skill, err := ParseContent(content)
	if err != nil {
		return nil, err
	}

	skill.Path = filepath.Dir(path)
	skill.SkillFilePath = path

	return skill, nil
}

// ParseContent parses a SKILL.md from raw bytes.
func ParseContent(content []byte) (*Skill, error) {
	frontmatter, body, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	skill.Instructions = strings.TrimSpace(body)

	return &skill, nil
}

// splitFrontmatter extracts YAML frontmatter and body from markdown content.
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	// Normalize line endings to \n for consistent parsing.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", "", errors.New("no YAML frontmatter found")
	}

	rest := strings.TrimPrefix(content, "---\n")
	before, after, ok := strings.Cut(rest, "\n---")
	if !ok {
		return "", "", errors.New("unclosed frontmatter")
	}

	return before, after, nil
}

// Discover finds all valid skills in the given paths.
func Discover(paths []string) []*Skill {
	var discovered []*Skill
	var mu sync.Mutex
	seen := make(map[string]bool)

	for _, base := range paths {
		// We use fastwalk with Follow: true instead of filepath.WalkDir because
		// WalkDir doesn't follow symlinked directories at any depth—only entry
		// points. This ensures skills in symlinked subdirectories are discovered.
		// fastwalk is concurrent, so we protect shared state (seen, skills) with mu.
		conf := fastwalk.Config{
			Follow:  true,
			ToSlash: fastwalk.DefaultToSlash(),
		}
		fastwalk.Walk(&conf, base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() || d.Name() != SkillFileName {
				return nil
			}
			mu.Lock()
			if seen[path] {
				mu.Unlock()
				return nil
			}
			seen[path] = true
			mu.Unlock()

			skill, err := Parse(path)
			if err != nil {
				slog.Warn("Failed to parse skill file", "path", path, "error", err)
				return nil
			}
			if err := skill.Validate(); err != nil {
				slog.Warn("Skill validation failed", "path", path, "error", err)
				return nil
			}
			slog.Debug("Successfully loaded skill", "name", skill.Name, "path", path)

			mu.Lock()
			discovered = append(discovered, skill)
			mu.Unlock()
			return nil
		})
	}

	return discovered
}

// Inspect parses SKILL.md files and returns safety outcomes for each parsed
// skill. Skills that cannot be parsed are skipped.
func Inspect(paths []string) []*Inspection {
	var out []*Inspection
	var mu sync.Mutex
	seen := make(map[string]bool)

	for _, base := range paths {
		conf := fastwalk.Config{
			Follow:  true,
			ToSlash: fastwalk.DefaultToSlash(),
		}
		fastwalk.Walk(&conf, base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() || d.Name() != SkillFileName {
				return nil
			}

			mu.Lock()
			if seen[path] {
				mu.Unlock()
				return nil
			}
			seen[path] = true
			mu.Unlock()

			skill, parseErr := Parse(path)
			if parseErr != nil {
				slog.Warn("Failed to parse skill for inspection", "path", path, "error", parseErr)
				return nil
			}

			name := strings.TrimSpace(skill.Name)
			if name == "" {
				name = filepath.Base(skill.Path)
			}
			reasons := skill.PromptInjectionDetectors()
			permissionWarnings := collectPermissionWarnings(skill.Path)
			inspection := &Inspection{
				Name:               name,
				Description:        strings.TrimSpace(skill.Description),
				Path:               skill.Path,
				SkillFilePath:      skill.SkillFilePath,
				Blocked:            len(reasons) > 0,
				BlockReasons:       reasons,
				PermissionWarnings: permissionWarnings,
			}

			mu.Lock()
			out = append(out, inspection)
			mu.Unlock()
			return nil
		})
	}

	return out
}

func collectPermissionWarnings(skillDir string) []string {
	const maxWarnings = 10
	warnings := make([]string, 0, maxWarnings)

	_ = filepath.WalkDir(skillDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if len(warnings) < maxWarnings {
				warnings = append(warnings, fmt.Sprintf("%s: access error (%v)", path, err))
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			if len(warnings) < maxWarnings {
				warnings = append(warnings, fmt.Sprintf("%s: stat error (%v)", path, infoErr))
			}
			return nil
		}

		perm := info.Mode().Perm()
		if perm != 0o600 && len(warnings) < maxWarnings {
			warnings = append(warnings, fmt.Sprintf("%s has %04o (expected 0600)", path, perm))
		}
		return nil
	})

	return warnings
}

// Audit validates every SKILL.md file under the given paths and returns an
// error if any skill is malformed or trips the prompt-injection safeguards.
func Audit(paths []string) error {
	var errs []error
	seen := make(map[string]bool)

	for _, base := range paths {
		conf := fastwalk.Config{
			Follow:  true,
			ToSlash: fastwalk.DefaultToSlash(),
		}
		fastwalk.Walk(&conf, base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() || d.Name() != SkillFileName {
				return nil
			}
			if seen[path] {
				return nil
			}
			seen[path] = true

			skill, parseErr := Parse(path)
			if parseErr != nil {
				errs = append(errs, fmt.Errorf("%s: parse failed: %w", path, parseErr))
				return nil
			}
			if validateErr := skill.Validate(); validateErr != nil {
				errs = append(errs, fmt.Errorf("%s: validation failed: %w", path, validateErr))
			}
			return nil
		})
	}

	return errors.Join(errs...)
}

// ToPromptXML generates XML for injection into the system prompt.
func ToPromptXML(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	for _, s := range skills {
		sb.WriteString("  <skill>\n")
		fmt.Fprintf(&sb, "    <name>%s</name>\n", escape(s.Name))
		fmt.Fprintf(&sb, "    <description>%s</description>\n", escape(s.Description))
		fmt.Fprintf(&sb, "    <location>%s</location>\n", escape(s.SkillFilePath))
		if s.Builtin {
			sb.WriteString("    <type>builtin</type>\n")
		}
		sb.WriteString("  </skill>\n")
	}
	sb.WriteString("</available_skills>")
	return sb.String()
}

func escape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}

func (s *Skill) PromptInjectionDetectors() []string {
	content := strings.TrimSpace(s.Instructions)
	if content == "" {
		return nil
	}

	matches := make([]string, 0, len(promptInjectionDetectors))
	for _, detector := range promptInjectionDetectors {
		if detector.Pattern.MatchString(content) {
			matches = append(matches, detector.Name)
		}
	}
	slices.Sort(matches)
	matches = slices.Compact(matches)
	if len(matches) == 0 {
		return nil
	}
	return matches
}

// Deduplicate removes duplicate skills by name. When duplicates exist, the
// last occurrence wins. This means user skills (appended after builtins)
// override builtin skills with the same name.
func Deduplicate(all []*Skill) []*Skill {
	seen := make(map[string]int, len(all))
	for i, s := range all {
		seen[s.Name] = i
	}

	result := make([]*Skill, 0, len(seen))
	for i, s := range all {
		if seen[s.Name] == i {
			result = append(result, s)
		}
	}
	return result
}

// Filter removes skills whose names appear in the disabled list.
func Filter(all []*Skill, disabled []string) []*Skill {
	if len(disabled) == 0 {
		return all
	}

	disabledSet := make(map[string]bool, len(disabled))
	for _, name := range disabled {
		disabledSet[name] = true
	}

	result := make([]*Skill, 0, len(all))
	for _, s := range all {
		if !disabledSet[s.Name] {
			result = append(result, s)
		}
	}
	return result
}
