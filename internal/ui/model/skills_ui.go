package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/marang/franz-agent/internal/config"
	"github.com/marang/franz-agent/internal/skills"
	"github.com/marang/franz-agent/internal/ui/dialog"
	"github.com/marang/franz-agent/internal/ui/util"
)

func (m *UI) listSkillsCmd() tea.Cmd {
	return func() tea.Msg {
		cfg := m.com.Config()
		if cfg == nil {
			return util.NewWarnMsg("Skills unavailable: configuration not found")
		}

		discovered := skills.Discover(cfg.Options.SkillsPaths)
		if len(discovered) == 0 {
			return util.NewInfoMsg("No skills discovered")
		}

		slices.SortFunc(discovered, func(a, b *skills.Skill) int {
			return strings.Compare(a.Name, b.Name)
		})

		const maxSkillsInStatus = 8
		names := make([]string, 0, min(len(discovered), maxSkillsInStatus))
		for i := range min(len(discovered), maxSkillsInStatus) {
			names = append(names, discovered[i].Name)
		}

		summary := fmt.Sprintf("Discovered %d skills: %s", len(discovered), strings.Join(names, ", "))
		if len(discovered) > maxSkillsInStatus {
			summary += ", ..."
		}
		return util.NewInfoMsg(summary)
	}
}

func (m *UI) skillsSHSearchCmd(query string, requestID int) tea.Cmd {
	return func() tea.Msg {
		query = strings.TrimSpace(query)
		if query == "" {
			return dialog.SkillsSHSearchResultsMsg{
				Query:     query,
				RequestID: requestID,
			}
		}

		output, err := m.runSelfCommand("skills", "sh", "search", "--json", query)
		if err != nil {
			return dialog.SkillsSHSearchResultsMsg{
				Query:     query,
				RequestID: requestID,
				Err:       err,
			}
		}

		var payload struct {
			Query   string `json:"query"`
			Results []struct {
				Name     string `json:"name"`
				Slug     string `json:"slug"`
				SkillID  string `json:"skillId"`
				Source   string `json:"source"`
				Installs int    `json:"installs"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(output), &payload); err != nil {
			return dialog.SkillsSHSearchResultsMsg{
				Query:     query,
				RequestID: requestID,
				Err:       fmt.Errorf("failed to parse search results: %w", err),
			}
		}

		results := make([]dialog.SkillsSHSearchResult, 0, len(payload.Results))
		for _, r := range payload.Results {
			results = append(results, dialog.SkillsSHSearchResult{
				Name:          r.Name,
				Source:        r.Source,
				Slug:          r.Slug,
				SkillID:       r.SkillID,
				Installs:      r.Installs,
				InstallSource: buildSkillsSHInstallSource(r.Source, r.Slug, r.SkillID, r.Name),
				DetailsURL:    buildSkillsSHDetailsURL(r.Source, r.Slug, r.SkillID),
			})
		}

		return dialog.SkillsSHSearchResultsMsg{
			Query:     query,
			RequestID: requestID,
			Results:   results,
		}
	}
}

func (m *UI) skillsSHInstallCmd(source string) tea.Cmd {
	return func() tea.Msg {
		output, err := m.runSelfCommand("skills", "sh", "install", source)
		if err != nil {
			return util.NewErrorMsg(err)
		}
		if output == "" {
			output = "Installed from skills.sh source: " + source
		}
		return util.NewInfoMsg(output)
	}
}

func (m *UI) skillsSHUpdateCmd(source string) tea.Cmd {
	return func() tea.Msg {
		args := []string{"skills", "sh", "update"}
		if source != "" {
			args = append(args, source)
		}
		output, err := m.runSelfCommand(args...)
		if err != nil {
			return util.NewErrorMsg(err)
		}
		if output == "" {
			output = "Updated skills.sh sources"
		}
		return util.NewInfoMsg(output)
	}
}

func (m *UI) skillsSHSourcesCmd() tea.Cmd {
	return func() tea.Msg {
		output, err := m.runSelfCommand("skills", "sh", "sources")
		if err != nil {
			return util.NewErrorMsg(err)
		}
		if output == "" {
			output = "No tracked skills.sh sources"
		}
		return util.NewInfoMsg(output)
	}
}

func (m *UI) skillsSHSourcesLoadCmd() tea.Cmd {
	return func() tea.Msg {
		output, err := m.runSelfCommand("skills", "sh", "sources")
		if err != nil {
			return dialog.SkillsSHSourcesLoadedMsg{Err: err}
		}
		return dialog.SkillsSHSourcesLoadedMsg{
			Sources: parseSkillsSHSourcesOutput(output),
		}
	}
}

func (m *UI) skillsSHInstallSelectedCmd(sources []string) tea.Cmd {
	return func() tea.Msg {
		installed := make([]string, 0, len(sources))
		failed := make(map[string]string)

		for _, source := range sources {
			source = strings.TrimSpace(source)
			if source == "" {
				continue
			}
			if _, err := m.runSelfCommand("skills", "sh", "install", source); err != nil {
				failed[source] = err.Error()
				continue
			}
			installed = append(installed, source)
		}

		return dialog.SkillsSHInstallCompletedMsg{
			Installed: installed,
			Failed:    failed,
		}
	}
}

func (m *UI) skillsSHInstallSourceCmd(source string) tea.Cmd {
	return func() tea.Msg {
		source = strings.TrimSpace(source)
		if source == "" {
			return dialog.SkillsSHInstallStepCompletedMsg{
				Source: source,
				Err:    errors.New("empty source"),
			}
		}
		_, err := m.runSelfCommand("skills", "sh", "install", source)
		return dialog.SkillsSHInstallStepCompletedMsg{
			Source: source,
			Err:    err,
		}
	}
}

func (m *UI) skillsInstalledLoadCmd() tea.Cmd {
	return func() tea.Msg {
		cfg := m.com.Config()
		if cfg == nil {
			return dialog.SkillsInstalledLoadedMsg{Err: errors.New("configuration not found")}
		}

		disabled := make(map[string]struct{}, len(cfg.Options.DisabledSkills))
		for _, name := range cfg.Options.DisabledSkills {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			disabled[name] = struct{}{}
		}

		inspections := skills.Inspect(cfg.Options.SkillsPaths)
		items := make([]dialog.SkillsInstalledItem, 0, len(inspections))
		for _, in := range inspections {
			if in == nil || strings.TrimSpace(in.Name) == "" {
				continue
			}
			_, isDisabled := disabled[in.Name]
			origin := readSkillOrigin(in.Path)
			items = append(items, dialog.SkillsInstalledItem{
				Name:               in.Name,
				Description:        in.Description,
				Path:               in.Path,
				SkillFile:          in.SkillFilePath,
				Disabled:           isDisabled,
				Blocked:            in.Blocked,
				BlockReasons:       slices.Clone(in.BlockReasons),
				PermissionWarnings: slices.Clone(in.PermissionWarnings),
				Origin:             origin,
			})
		}

		slices.SortFunc(items, func(a, b dialog.SkillsInstalledItem) int {
			return strings.Compare(a.Name, b.Name)
		})
		return dialog.SkillsInstalledLoadedMsg{Items: items}
	}
}

func (m *UI) setSkillDisabledCmd(name string, disabled bool) tea.Cmd {
	return func() tea.Msg {
		name = strings.TrimSpace(name)
		if name == "" {
			return util.NewWarnMsg("Skill name is required")
		}

		cfg := m.com.Config()
		if cfg == nil {
			return util.NewErrorMsg(errors.New("configuration not found"))
		}

		current := slices.Clone(cfg.Options.DisabledSkills)
		current = slices.DeleteFunc(current, func(v string) bool {
			return strings.EqualFold(strings.TrimSpace(v), name)
		})
		if disabled {
			current = append(current, name)
		}
		slices.Sort(current)
		current = slices.Compact(current)

		if err := m.com.Store().SetConfigField(config.ScopeGlobal, "options.disabled_skills", current); err != nil {
			return util.NewErrorMsg(err)
		}
		cfg.Options.DisabledSkills = current

		if disabled {
			return util.NewInfoMsg("Skill disabled: " + name)
		}
		return util.NewInfoMsg("Skill enabled: " + name)
	}
}

func (m *UI) fixSkillPermissionsCmd(names []string) tea.Cmd {
	return func() tea.Msg {
		if runtime.GOOS == "windows" {
			return util.NewWarnMsg("Permission fix is currently supported on Unix-like systems only")
		}

		cfg := m.com.Config()
		if cfg == nil {
			return util.NewWarnMsg("Cannot fix permissions: configuration not found")
		}

		inspections := skills.Inspect(cfg.Options.SkillsPaths)
		selected := make(map[string]struct{}, len(names))
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n != "" {
				selected[n] = struct{}{}
			}
		}
		changed := 0
		failed := 0

		for _, in := range inspections {
			if in == nil || strings.TrimSpace(in.Path) == "" {
				continue
			}
			if len(selected) > 0 {
				if _, ok := selected[in.Name]; !ok {
					continue
				}
			}
			_ = filepath.WalkDir(in.Path, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					failed++
					return nil
				}
				var chmodErr error
				if d.IsDir() {
					chmodErr = os.Chmod(path, 0o700)
				} else {
					chmodErr = os.Chmod(path, 0o600)
				}
				if chmodErr != nil {
					failed++
					return nil
				}
				changed++
				return nil
			})
		}

		if failed > 0 {
			return util.NewWarnMsg(fmt.Sprintf("Permissions fixed for %d entries, %d failed", changed, failed))
		}
		return util.NewInfoMsg(fmt.Sprintf("Permissions fixed for %d entries", changed))
	}
}

func (m *UI) setSkillsDisabledCmd(names []string, disabled bool) tea.Cmd {
	return func() tea.Msg {
		cfg := m.com.Config()
		if cfg == nil || cfg.Options == nil {
			return util.NewWarnMsg("Skill update unavailable: configuration not found")
		}

		unique := make(map[string]struct{}, len(names))
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name != "" {
				unique[name] = struct{}{}
			}
		}
		if len(unique) == 0 {
			return util.NewWarnMsg("No installed skills selected")
		}

		current := slices.Clone(cfg.Options.DisabledSkills)
		if current == nil {
			current = []string{}
		}
		set := make(map[string]struct{}, len(current))
		for _, n := range current {
			n = strings.TrimSpace(n)
			if n != "" {
				set[n] = struct{}{}
			}
		}

		for name := range unique {
			if disabled {
				set[name] = struct{}{}
			} else {
				delete(set, name)
			}
		}

		next := make([]string, 0, len(set))
		for name := range set {
			next = append(next, name)
		}
		slices.Sort(next)
		next = slices.Compact(next)

		if err := m.com.Store().SetConfigField(config.ScopeGlobal, "options.disabled_skills", next); err != nil {
			return util.NewErrorMsg(err)
		}
		cfg.Options.DisabledSkills = next

		action := "enabled"
		if disabled {
			action = "disabled"
		}
		return util.NewInfoMsg(fmt.Sprintf("%d skill(s) %s", len(unique), action))
	}
}

func (m *UI) openExternalURLCmd(rawURL string) tea.Cmd {
	return func() tea.Msg {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			return util.NewWarnMsg("No URL available")
		}
		if err := openURLSilently(rawURL); err != nil {
			return util.NewErrorMsg(fmt.Errorf("failed to open URL: %w", err))
		}
		return util.NewInfoMsg("Opened details URL")
	}
}

func openURLSilently(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(context.Background(), "open", rawURL)
	case "windows":
		cmd = exec.CommandContext(context.Background(), "rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.CommandContext(context.Background(), "xdg-open", rawURL)
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func parseSkillsSHSourcesOutput(output string) []string {
	output = strings.TrimSpace(output)
	if output == "" || output == "No tracked skills.sh sources." {
		return nil
	}

	lines := strings.Split(output, "\n")
	sources := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "updated ") {
			continue
		}
		sources = append(sources, line)
	}
	return sources
}

func buildSkillsSHInstallSource(source, slug, skillID, name string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}

	skill := strings.TrimSpace(slug)
	if skill == "" {
		// skillId can be opaque and not match repository skill names; prefer
		// human-readable name before falling back.
		skill = strings.TrimSpace(name)
	}
	if skill == "" {
		skill = strings.TrimSpace(skillID)
	}
	if skill == "" {
		return "skills.sh/" + source
	}
	return "skills.sh/" + source + "/" + skill
}

func buildSkillsSHDetailsURL(source, slug, skillID string) string {
	source = strings.Trim(strings.TrimSpace(source), "/")
	if source == "" {
		return ""
	}
	path := source
	skill := strings.TrimSpace(slug)
	if skill == "" {
		skill = strings.TrimSpace(skillID)
	}
	if skill != "" {
		path += "/" + skill
	}
	return "https://skills.sh/" + path
}

type skillOriginMetadata struct {
	Source string `json:"source"`
}

func readSkillOrigin(skillPath string) string {
	skillPath = strings.TrimSpace(skillPath)
	if skillPath == "" {
		return ""
	}
	originFile := filepath.Join(skillPath, ".franz-origin.json")
	data, err := os.ReadFile(originFile)
	if err != nil {
		return ""
	}
	var md skillOriginMetadata
	if err := json.Unmarshal(data, &md); err != nil {
		return ""
	}
	return strings.TrimSpace(md.Source)
}

func (m *UI) deleteSkillsCmd(names []string) tea.Cmd {
	return func() tea.Msg {
		cfg := m.com.Config()
		if cfg == nil || cfg.Options == nil {
			return util.NewWarnMsg("Skill delete unavailable: configuration not found")
		}

		selected := make(map[string]struct{}, len(names))
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name != "" {
				selected[name] = struct{}{}
			}
		}
		if len(selected) == 0 {
			return util.NewWarnMsg("No installed skills selected")
		}

		inspections := skills.Inspect(cfg.Options.SkillsPaths)
		if len(inspections) == 0 {
			return util.NewWarnMsg("No installed skills found")
		}

		deleted := 0
		failed := 0
		seen := make(map[string]struct{}, len(selected))
		for _, in := range inspections {
			if in == nil {
				continue
			}
			name := strings.TrimSpace(in.Name)
			if name == "" {
				continue
			}
			if _, ok := selected[name]; !ok {
				continue
			}
			seen[name] = struct{}{}
			path := strings.TrimSpace(in.Path)
			if path == "" {
				failed++
				continue
			}
			if err := os.RemoveAll(path); err != nil {
				failed++
				continue
			}
			deleted++
		}

		notFound := 0
		for name := range selected {
			if _, ok := seen[name]; !ok {
				notFound++
			}
		}

		// Keep disabled list clean for deleted skills.
		current := slices.Clone(cfg.Options.DisabledSkills)
		next := slices.DeleteFunc(current, func(v string) bool {
			_, ok := selected[strings.TrimSpace(v)]
			return ok
		})
		slices.Sort(next)
		next = slices.Compact(next)
		if err := m.com.Store().SetConfigField(config.ScopeGlobal, "options.disabled_skills", next); err != nil {
			return util.NewErrorMsg(err)
		}
		cfg.Options.DisabledSkills = next

		if failed > 0 || notFound > 0 {
			return util.NewWarnMsg(fmt.Sprintf("Deleted %d skill(s), %d failed, %d not found", deleted, failed, notFound))
		}
		return util.NewInfoMsg(fmt.Sprintf("Deleted %d skill(s)", deleted))
	}
}
