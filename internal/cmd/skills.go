package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/term"
	"github.com/marang/franz-agent/internal/config"
	"github.com/marang/franz-agent/internal/skills"
	"github.com/spf13/cobra"
)

const (
	scopeFlagName         = "scope"
	skillsSHRegistryFile  = "skills-sh-sources.json"
	skillsSHCacheDir      = "skills-sh-cache"
	skillsSHSearchAPIBase = "https://skills.sh"
	skillsSHSearchLimit   = 10
	skillOriginFileName   = ".franz-origin.json"
)

const (
	promptInjectionMarker          = "prompt-injection safeguard triggered:"
	detectorIgnoreInstructions     = "ignore_instructions"
	detectorPromptExfiltration     = "prompt_exfiltration"
	detectorAuthorityImpersonation = "authority_impersonation"
	detectorToolBypass             = "tool_bypass"
	detectorDataExfiltration       = "data_exfiltration"
)

var promptInjectionDetectorReasons = map[string]string{
	detectorIgnoreInstructions:     "ignore_instructions (asks the model to ignore/bypass higher-priority instructions)",
	detectorPromptExfiltration:     "prompt_exfiltration (attempts to reveal hidden/system instructions or secrets)",
	detectorAuthorityImpersonation: "authority_impersonation (tries to re-define agent authority/role)",
	detectorToolBypass:             "tool_bypass (requests command/tool execution without approval)",
	detectorDataExfiltration:       "data_exfiltration (attempts to send secrets/credentials externally)",
}

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage Agent Skills paths and discovery",
	Long:  "Manage Agent Skills directories and list discovered skills.",
	RunE:  runSkillsList,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered skills",
	RunE:  runSkillsList,
}

var skillsPathsCmd = &cobra.Command{
	Use:   "paths",
	Short: "Show configured and effective skills paths",
	RunE:  runSkillsPaths,
}

var skillsAddPathCmd = &cobra.Command{
	Use:   "add-path <path>",
	Short: "Add a skills path to config",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsAddPath,
}

var skillsRemovePathCmd = &cobra.Command{
	Use:   "remove-path <path>",
	Short: "Remove a skills path from config",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsRemovePath,
}

var skillsSHCmd = &cobra.Command{
	Use:   "sh",
	Short: "Integrate with skills.sh",
}

var skillsSHInstallCmd = &cobra.Command{
	Use:   "install <source>",
	Short: "Install a skill source from skills.sh",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsSHInstall,
}

var skillsSHUpdateCmd = &cobra.Command{
	Use:   "update [source]",
	Short: "Update tracked skills.sh sources (or one source)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSkillsSHUpdate,
}

var skillsSHSourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "List tracked skills.sh sources",
	RunE:  runSkillsSHSources,
}

var skillsSHSearchCmd = &cobra.Command{
	Use:     "search <query>",
	Aliases: []string{"find"},
	Short:   "Search skills available on skills.sh",
	Args:    cobra.MinimumNArgs(1),
	RunE:    runSkillsSHSearch,
}

func init() {
	skillsListCmd.Flags().Bool("json", false, "Output as JSON")
	skillsPathsCmd.Flags().Bool("json", false, "Output as JSON")
	skillsSHSearchCmd.Flags().Bool("json", false, "Output as JSON")

	skillsAddPathCmd.Flags().String(scopeFlagName, "global", "Config scope: global or workspace")
	skillsRemovePathCmd.Flags().String(scopeFlagName, "global", "Config scope: global or workspace")

	skillsCmd.AddCommand(
		skillsListCmd,
		skillsPathsCmd,
		skillsAddPathCmd,
		skillsRemovePathCmd,
	)
	skillsSHCmd.AddCommand(
		skillsSHInstallCmd,
		skillsSHUpdateCmd,
		skillsSHSourcesCmd,
		skillsSHSearchCmd,
	)
	skillsCmd.AddCommand(skillsSHCmd)

	rootCmd.AddCommand(skillsCmd)
}

func runSkillsList(cmd *cobra.Command, _ []string) error {
	store, err := initConfigStore(cmd)
	if err != nil {
		return err
	}

	effectivePaths := store.Config().Options.SkillsPaths
	discovered := skills.Discover(effectivePaths)
	slices.SortFunc(discovered, func(a, b *skills.Skill) int {
		return strings.Compare(a.Name, b.Name)
	})

	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		output := struct {
			Paths  []string        `json:"paths"`
			Skills []*skills.Skill `json:"skills"`
		}{
			Paths:  effectivePaths,
			Skills: discovered,
		}
		b, err := json.Marshal(output)
		if err != nil {
			return err
		}
		cmd.Println(string(b))
		return nil
	}

	if len(discovered) == 0 {
		cmd.Println("No skills discovered.")
		return nil
	}

	if term.IsTerminal(os.Stdout.Fd()) {
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			StyleFunc(func(row, col int) lipgloss.Style {
				return lipgloss.NewStyle().Padding(0, 1)
			}).
			Headers("Name", "Description", "Path")

		for _, s := range discovered {
			t.Row(s.Name, s.Description, s.SkillFilePath)
		}
		lipgloss.Println(t)
		return nil
	}

	for _, s := range discovered {
		cmd.Printf("%s\t%s\t%s\n", s.Name, s.Description, s.SkillFilePath)
	}
	return nil
}

func runSkillsPaths(cmd *cobra.Command, _ []string) error {
	store, err := initConfigStore(cmd)
	if err != nil {
		return err
	}

	configuredGlobal, err := store.SkillsPaths(config.ScopeGlobal)
	if err != nil {
		return err
	}
	configuredWorkspace, err := store.SkillsPaths(config.ScopeWorkspace)
	if err != nil {
		return err
	}

	effectivePaths := store.Config().Options.SkillsPaths
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		output := struct {
			Configured struct {
				Global    []string `json:"global"`
				Workspace []string `json:"workspace"`
			} `json:"configured"`
			Effective []string `json:"effective"`
		}{Effective: effectivePaths}
		output.Configured.Global = configuredGlobal
		output.Configured.Workspace = configuredWorkspace

		b, err := json.Marshal(output)
		if err != nil {
			return err
		}
		cmd.Println(string(b))
		return nil
	}

	printPaths := func(title string, paths []string) {
		cmd.Println(title + ":")
		if len(paths) == 0 {
			cmd.Println("  (none)")
			return
		}
		for _, p := range paths {
			cmd.Println("  - " + p)
		}
	}

	printPaths("Configured (global)", configuredGlobal)
	printPaths("Configured (workspace)", configuredWorkspace)
	printPaths("Effective (discovery order)", effectivePaths)
	return nil
}

func runSkillsAddPath(cmd *cobra.Command, args []string) error {
	scope, err := parseScopeFlag(cmd)
	if err != nil {
		return err
	}

	store, err := initConfigStore(cmd)
	if err != nil {
		return err
	}

	cwd := store.WorkingDir()
	targetPath, err := normalizeSkillPath(cwd, args[0])
	if err != nil {
		return err
	}

	configured, err := store.SkillsPaths(scope)
	if err != nil {
		return err
	}

	if slices.Contains(configured, targetPath) {
		cmd.Printf("skills path already configured: %s\n", targetPath)
		return nil
	}

	configured = append(configured, targetPath)
	if err := store.SetConfigField(scope, "options.skills_paths", configured); err != nil {
		return err
	}

	cmd.Printf("added skills path (%s): %s\n", scopeLabel(scope), targetPath)
	return nil
}

func runSkillsRemovePath(cmd *cobra.Command, args []string) error {
	scope, err := parseScopeFlag(cmd)
	if err != nil {
		return err
	}

	store, err := initConfigStore(cmd)
	if err != nil {
		return err
	}

	cwd := store.WorkingDir()
	targetPath, err := normalizeSkillPath(cwd, args[0])
	if err != nil {
		return err
	}

	configured, err := store.SkillsPaths(scope)
	if err != nil {
		return err
	}
	if len(configured) == 0 {
		return fmt.Errorf("no configured skills paths in %s scope", scopeLabel(scope))
	}

	filtered := configured[:0]
	found := false
	for _, p := range configured {
		if p == targetPath {
			found = true
			continue
		}
		filtered = append(filtered, p)
	}
	if !found {
		return fmt.Errorf("skills path not found in %s scope: %s", scopeLabel(scope), targetPath)
	}

	if len(filtered) == 0 {
		if err := store.RemoveConfigField(scope, "options.skills_paths"); err != nil {
			return err
		}
	} else if err := store.SetConfigField(scope, "options.skills_paths", filtered); err != nil {
		return err
	}

	cmd.Printf("removed skills path (%s): %s\n", scopeLabel(scope), targetPath)
	return nil
}

func runSkillsSHInstall(cmd *cobra.Command, args []string) error {
	source := strings.TrimSpace(args[0])
	if source == "" {
		return fmt.Errorf("source cannot be empty")
	}

	store, err := initConfigStore(cmd)
	if err != nil {
		return err
	}

	if err := installSkillsSHSource(cmd, store, source); err != nil {
		return err
	}
	if err := trackSkillsSHSource(store, source); err != nil {
		return err
	}

	cmd.Printf("installed from skills.sh source: %s\n", source)
	return nil
}

func runSkillsSHUpdate(cmd *cobra.Command, args []string) error {
	store, err := initConfigStore(cmd)
	if err != nil {
		return err
	}

	sources := []string{}
	if len(args) == 1 {
		source := strings.TrimSpace(args[0])
		if source == "" {
			return fmt.Errorf("source cannot be empty")
		}
		sources = append(sources, source)
	} else {
		sources, err = trackedSkillsSHSources(store)
		if err != nil {
			return err
		}
		if len(sources) == 0 {
			return fmt.Errorf("no tracked skills.sh sources; use `franz-agent skills sh install <source>` first")
		}
	}

	var failed []string
	for _, source := range sources {
		if err := installSkillsSHSource(cmd, store, source); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", source, err))
			continue
		}
		if err := trackSkillsSHSource(store, source); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", source, err))
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("skills.sh update completed with errors: %s", strings.Join(failed, "; "))
	}

	cmd.Printf("updated %d skills.sh source(s)\n", len(sources))
	return nil
}

func runSkillsSHSources(cmd *cobra.Command, _ []string) error {
	store, err := initConfigStore(cmd)
	if err != nil {
		return err
	}

	sources, err := trackedSkillsSHSources(store)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		cmd.Println("No tracked skills.sh sources.")
		return nil
	}

	for _, source := range sources {
		cmd.Println(source)
	}
	return nil
}

func runSkillsSHSearch(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return fmt.Errorf("query cannot be empty")
	}

	results, err := searchSkillsSH(cmd.Context(), query, skillsSHSearchLimit)
	if err != nil {
		return err
	}

	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		output := struct {
			Query   string                 `json:"query"`
			Results []skillsSHSearchResult `json:"results"`
		}{
			Query:   query,
			Results: results,
		}
		data, err := json.Marshal(output)
		if err != nil {
			return err
		}
		cmd.Println(string(data))
		return nil
	}

	if len(results) == 0 {
		cmd.Println("No skills found on skills.sh.")
		return nil
	}

	if term.IsTerminal(os.Stdout.Fd()) {
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			StyleFunc(func(row, col int) lipgloss.Style {
				return lipgloss.NewStyle().Padding(0, 1)
			}).
			Headers("Name", "Source", "Installs", "Install Source")
		for _, result := range results {
			t.Row(result.Name, result.Source, strconv.Itoa(result.Installs), result.InstallSource())
		}
		lipgloss.Println(t)
		return nil
	}

	for _, result := range results {
		cmd.Printf("%s\t%s\t%d\t%s\n", result.Name, result.Source, result.Installs, result.InstallSource())
	}
	return nil
}

func initConfigStore(cmd *cobra.Command) (*config.ConfigStore, error) {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return nil, err
	}

	dataDir, _ := cmd.Flags().GetString("data-dir")
	debug, _ := cmd.Flags().GetBool("debug")
	return config.Init(cwd, dataDir, debug)
}

func parseScopeFlag(cmd *cobra.Command) (config.Scope, error) {
	value, _ := cmd.Flags().GetString(scopeFlagName)
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "global":
		return config.ScopeGlobal, nil
	case "workspace":
		return config.ScopeWorkspace, nil
	default:
		return config.ScopeGlobal, fmt.Errorf("invalid scope %q (expected: global|workspace)", value)
	}
}

func scopeLabel(scope config.Scope) string {
	if scope == config.ScopeWorkspace {
		return "workspace"
	}
	return "global"
}

func normalizeSkillPath(cwd, p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	p = os.ExpandEnv(p)
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, p[2:])
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	return filepath.Clean(p), nil
}

func installSkillsSHSource(cmd *cobra.Command, store *config.ConfigStore, source string) error {
	spec, err := parseSkillsSHSource(source)
	if err != nil {
		return err
	}

	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is required for skills.sh integration: %w", err)
	}

	cacheRoot := filepath.Join(store.Config().Options.DataDirectory, skillsSHCacheDir)
	cachePath := filepath.Join(cacheRoot, spec.CacheKey)
	if err := ensureRepoCheckout(cmd, cachePath, spec.RepoURL); err != nil {
		return err
	}

	if spec.SkillName == "" {
		if err := skills.Audit([]string{cachePath}); err != nil {
			return formatSkillsAuditError(err)
		}
	} else {
		inspected := skills.Inspect([]string{cachePath})
		targetPaths := make([]string, 0, 1)
		for _, in := range inspected {
			if in == nil {
				continue
			}
			if matchesSkillIdentifier(spec.SkillName, &skills.Skill{
				Name: in.Name,
				Path: in.Path,
			}) {
				targetPaths = append(targetPaths, in.Path)
			}
		}
		if len(targetPaths) == 0 {
			return fmt.Errorf("skill %q not found in source repo %s", spec.SkillName, spec.RepoURL)
		}
		if err := skills.Audit(targetPaths); err != nil {
			return formatSkillsAuditError(err)
		}
	}

	discovered := skills.Discover([]string{cachePath})
	if len(discovered) == 0 {
		return fmt.Errorf("no skills found in source repo: %s", spec.RepoURL)
	}

	if spec.SkillName != "" {
		discovered = slices.DeleteFunc(discovered, func(s *skills.Skill) bool {
			return !matchesSkillIdentifier(spec.SkillName, s)
		})
		if len(discovered) == 0 {
			available := make([]string, 0, len(skills.Discover([]string{cachePath})))
			for _, s := range skills.Discover([]string{cachePath}) {
				if s != nil && strings.TrimSpace(s.Name) != "" {
					available = append(available, s.Name)
				}
			}
			slices.Sort(available)
			if len(available) > 0 {
				return fmt.Errorf("skill %q not found in source repo %s (available: %s)", spec.SkillName, spec.RepoURL, strings.Join(available, ", "))
			}
			return fmt.Errorf("skill %q not found in source repo %s", spec.SkillName, spec.RepoURL)
		}
	}

	targetRoot := config.GlobalSkillsDirs()[0]
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return fmt.Errorf("failed to create skills target dir: %w", err)
	}

	for _, skill := range discovered {
		if err := skill.Validate(); err != nil {
			return fmt.Errorf("skill %q failed validation: %w", skill.Name, err)
		}
		targetPath := filepath.Join(targetRoot, skill.Name)
		if err := replaceWithSymlinkOrCopy(skill.Path, targetPath); err != nil {
			return err
		}
		if err := writeSkillOrigin(targetPath, "skills.sh/"+spec.Source); err != nil {
			return err
		}
	}

	return nil
}

func ensureRepoCheckout(cmd *cobra.Command, cachePath, repoURL string) error {
	if _, err := os.Stat(filepath.Join(cachePath, ".git")); err == nil {
		fetch := exec.CommandContext(cmd.Context(), "git", "-C", cachePath, "pull", "--ff-only")
		fetch.Stdout = cmd.OutOrStdout()
		fetch.Stderr = cmd.ErrOrStderr()
		if err := fetch.Run(); err != nil {
			return fmt.Errorf("failed to update cached skills repo: %w", err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		return fmt.Errorf("failed to create skills cache dir: %w", err)
	}

	clone := exec.CommandContext(cmd.Context(), "git", "clone", "--depth", "1", repoURL, cachePath)
	clone.Stdout = cmd.OutOrStdout()
	clone.Stderr = cmd.ErrOrStderr()
	if err := clone.Run(); err != nil {
		return fmt.Errorf("failed to clone skills source repo: %w", err)
	}
	if err := secureTreePermissions(cachePath); err != nil {
		return fmt.Errorf("failed to secure cached skills repo permissions: %w", err)
	}
	return nil
}

type skillsSourceSpec struct {
	RepoURL   string
	SkillName string
	CacheKey  string
	Source    string
}

type skillsSHSearchResult struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	SkillID  string `json:"skillId"`
	ID       string `json:"id"`
	Source   string `json:"source"`
	Installs int    `json:"installs"`
}

func (r skillsSHSearchResult) InstallSource() string {
	if r.Source == "" {
		return ""
	}
	skill := strings.TrimSpace(r.Slug)
	if skill == "" {
		skill = strings.TrimSpace(r.SkillID)
	}
	if skill == "" {
		skill = strings.TrimSpace(r.Name)
	}
	if skill == "" {
		return "skills.sh/" + r.Source
	}
	return "skills.sh/" + r.Source + "/" + skill
}

func searchSkillsSH(ctx context.Context, query string, limit int) ([]skillsSHSearchResult, error) {
	baseURL := strings.TrimSpace(os.Getenv("SKILLS_API_URL"))
	if baseURL == "" {
		baseURL = skillsSHSearchAPIBase
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	endpoint, err := url.Parse(baseURL + "/api/search")
	if err != nil {
		return nil, fmt.Errorf("invalid skills search api URL: %w", err)
	}
	params := endpoint.Query()
	trimmedQuery := strings.TrimSpace(query)
	params.Set("q", trimmedQuery)
	params.Set("term", trimmedQuery)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create skills search request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search skills.sh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("skills.sh search failed: %s", message)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read skills.sh search response: %w", err)
	}

	var results []skillsSHSearchResult
	if err := json.Unmarshal(body, &results); err != nil {
		var wrapped struct {
			Skills []skillsSHSearchResult `json:"skills"`
		}
		if err := json.Unmarshal(body, &wrapped); err != nil {
			return nil, fmt.Errorf("failed to decode skills.sh search response: %w", err)
		}
		results = wrapped.Skills
	}

	slices.SortFunc(results, func(a, b skillsSHSearchResult) int {
		if a.Installs == b.Installs {
			return strings.Compare(a.Name, b.Name)
		}
		if a.Installs > b.Installs {
			return -1
		}
		return 1
	})

	return results, nil
}

func parseSkillsSHSource(source string) (skillsSourceSpec, error) {
	source = strings.TrimSpace(source)
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")
	source = strings.TrimPrefix(source, "skills.sh/")
	source = strings.TrimPrefix(source, "github.com/")
	source = strings.TrimPrefix(source, "https://github.com/")
	source = strings.TrimPrefix(source, "http://github.com/")

	source = strings.TrimPrefix(source, "/")
	source = strings.TrimSuffix(source, ".git")

	parts := strings.Split(source, "/")
	if len(parts) < 2 {
		return skillsSourceSpec{}, fmt.Errorf("invalid skills source %q (expected skills.sh/<owner>/<repo>[/<skill>])", source)
	}

	owner, repo := parts[0], parts[1]
	spec := skillsSourceSpec{
		RepoURL:  fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
		CacheKey: owner + "__" + repo,
	}
	if len(parts) >= 3 {
		spec.SkillName = strings.Join(parts[2:], "/")
		spec.Source += "/" + spec.SkillName
	}
	return spec, nil
}

func matchesSkillIdentifier(identifier string, s *skills.Skill) bool {
	if s == nil {
		return false
	}
	id := normalizeSkillIdentifier(identifier)
	if id == "" {
		return false
	}
	candidates := []string{
		s.Name,
		filepath.Base(s.Path),
	}
	for _, c := range candidates {
		if normalizeSkillIdentifier(c) == id {
			return true
		}
	}
	return false
}

func normalizeSkillIdentifier(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	v = strings.ReplaceAll(v, "_", "-")
	v = strings.ReplaceAll(v, " ", "-")
	v = strings.Trim(v, "-")
	return v
}

func formatSkillsAuditError(err error) error {
	msg := strings.TrimSpace(err.Error())
	lower := strings.ToLower(msg)
	idx := strings.Index(lower, promptInjectionMarker)
	if idx == -1 {
		return fmt.Errorf("skills source failed security audit: %s", msg)
	}

	detectorPart := strings.TrimSpace(msg[idx+len(promptInjectionMarker):])
	if detectorPart == "" {
		return fmt.Errorf("skills source failed security audit: %s", msg)
	}

	detectors := strings.Split(detectorPart, ",")
	explanations := make([]string, 0, len(detectors))
	for _, d := range detectors {
		name := strings.TrimSpace(d)
		if name == "" {
			continue
		}
		explanations = append(explanations, describePromptInjectionDetector(name))
	}
	if len(explanations) == 0 {
		return fmt.Errorf("skills source failed security audit: %s", msg)
	}

	return fmt.Errorf("skills source failed security audit: %s. Why blocked: %s", msg, strings.Join(explanations, "; "))
}

func describePromptInjectionDetector(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if reason, ok := promptInjectionDetectorReasons[key]; ok {
		return reason
	}
	return name
}

func replaceWithSymlinkOrCopy(srcDir, dstDir string) error {
	if err := os.RemoveAll(dstDir); err != nil {
		return fmt.Errorf("failed to replace existing skill dir: %w", err)
	}
	// Always copy with restrictive permissions for installed skill files.
	if err := copyDir(srcDir, dstDir); err != nil {
		return fmt.Errorf("failed to install skill: %w", err)
	}
	return nil
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}

	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		return err
	})
}

func secureTreePermissions(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Chmod(path, 0o700)
		}
		return os.Chmod(path, 0o600)
	})
}

func runCommand(cmd *cobra.Command, name string, args ...string) error {
	c := exec.CommandContext(cmd.Context(), name, args...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	c.Stdin = cmd.InOrStdin()
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to run `%s %s`: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func skillsSHRegistryPath(store *config.ConfigStore) string {
	return filepath.Join(store.Config().Options.DataDirectory, skillsSHRegistryFile)
}

type trackedSkillsSHSource struct {
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
}

func trackedSkillsSHSources(store *config.ConfigStore) ([]string, error) {
	path := skillsSHRegistryPath(store)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read skills.sh registry: %w", err)
	}

	var tracked []trackedSkillsSHSource
	if err := json.Unmarshal(data, &tracked); err != nil {
		return nil, fmt.Errorf("failed to parse skills.sh registry: %w", err)
	}

	sources := make([]string, 0, len(tracked))
	for _, item := range tracked {
		if item.Source != "" {
			sources = append(sources, item.Source)
		}
	}
	slices.Sort(sources)
	return slices.Compact(sources), nil
}

func trackSkillsSHSource(store *config.ConfigStore, source string) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}

	path := skillsSHRegistryPath(store)
	var tracked []trackedSkillsSHSource
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &tracked); err != nil {
			return fmt.Errorf("failed to parse skills.sh registry: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read skills.sh registry: %w", err)
	}

	found := false
	now := time.Now().UTC()
	for i := range tracked {
		if tracked[i].Source == source {
			tracked[i].UpdatedAt = now
			found = true
			break
		}
	}
	if !found {
		tracked = append(tracked, trackedSkillsSHSource{
			Source:    source,
			UpdatedAt: now,
		})
	}

	slices.SortFunc(tracked, func(a, b trackedSkillsSHSource) int {
		return strings.Compare(a.Source, b.Source)
	})

	data, err := json.MarshalIndent(tracked, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize skills.sh registry: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create skills.sh registry dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write skills.sh registry: %w", err)
	}

	return nil
}

func writeSkillOrigin(targetPath, source string) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}
	payload, err := json.MarshalIndent(map[string]string{
		"source": source,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize skill origin metadata: %w", err)
	}
	originPath := filepath.Join(targetPath, skillOriginFileName)
	if err := os.WriteFile(originPath, payload, 0o600); err != nil {
		return fmt.Errorf("failed to write skill origin metadata: %w", err)
	}
	return nil
}
