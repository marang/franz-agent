package dialog

import "testing"

func TestIsSkillsSourceTracked(t *testing.T) {
	t.Parallel()

	tracked := map[string]struct{}{
		"skills.sh/owner/repo":       {},
		"skills.sh/owner/repo/skill": {},
	}

	tests := []struct {
		name          string
		installSource string
		want          bool
	}{
		{
			name:          "exact skill match",
			installSource: "skills.sh/owner/repo/skill",
			want:          true,
		},
		{
			name:          "repo tracked covers skill",
			installSource: "skills.sh/owner/repo/other-skill",
			want:          true,
		},
		{
			name:          "skill tracked does not cover different repo",
			installSource: "skills.sh/owner/other/skill",
			want:          false,
		},
		{
			name:          "empty source",
			installSource: "",
			want:          false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isSkillsSourceTracked(tt.installSource, tracked)
			if got != tt.want {
				t.Fatalf("isSkillsSourceTracked(%q) = %v, want %v", tt.installSource, got, tt.want)
			}
		})
	}
}

func TestSkillsSHInstallStepCompletedRequestsRefreshOnFinalStep(t *testing.T) {
	t.Parallel()

	s := &SkillsSHSearch{
		selected:     map[string]struct{}{"skills.sh/owner/repo/skill": {}},
		tracked:      map[string]struct{}{},
		installQueue: nil,
		installTotal: 1,
		loading:      true,
	}

	action := s.HandleMsg(SkillsSHInstallStepCompletedMsg{
		Source: "skills.sh/owner/repo/skill",
		Err:    nil,
	})

	if _, ok := action.(ActionSkillsInstalledRefreshRequest); !ok {
		t.Fatalf("expected ActionSkillsInstalledRefreshRequest, got %T", action)
	}
	if s.loading {
		t.Fatal("expected loading=false after final install step")
	}
	if _, ok := s.tracked["skills.sh/owner/repo/skill"]; !ok {
		t.Fatal("expected installed source to be tracked")
	}
	if _, ok := s.selected["skills.sh/owner/repo/skill"]; ok {
		t.Fatal("expected installed source to be removed from selected set")
	}
}

func TestSkillsSHInstallStepCompletedContinuesQueue(t *testing.T) {
	t.Parallel()

	s := &SkillsSHSearch{
		installQueue: []string{"skills.sh/owner/repo/next"},
		installTotal: 2,
		loading:      true,
		selected:     map[string]struct{}{},
		tracked:      map[string]struct{}{},
	}

	action := s.HandleMsg(SkillsSHInstallStepCompletedMsg{
		Source: "skills.sh/owner/repo/current",
		Err:    nil,
	})

	msg, ok := action.(ActionSkillsSHInstallSource)
	if !ok {
		t.Fatalf("expected ActionSkillsSHInstallSource, got %T", action)
	}
	if msg.Source != "skills.sh/owner/repo/next" {
		t.Fatalf("unexpected next source: %q", msg.Source)
	}
	if !s.loading {
		t.Fatal("expected loading to remain true while queue continues")
	}
}

func TestSearchInstalledBadgeFollowsInstalledItemsNotTrackedSources(t *testing.T) {
	t.Parallel()

	const installSource = "skills.sh/owner/repo/skill"

	s := &SkillsSHSearch{
		selected:         map[string]struct{}{},
		tracked:          map[string]struct{}{"skills.sh/owner/repo": {}},
		installedSources: map[string]struct{}{},
		items: []*skillsSHSearchItem{{
			result: SkillsSHSearchResult{InstallSource: installSource},
		}},
	}

	// Tracked source alone must not mark a search result as installed.
	s.syncItemState()
	if s.items[0].installed {
		t.Fatal("expected search item to not be installed when only source is tracked")
	}

	// Installed origins should control the installed badge.
	s.installedSources[installSource] = struct{}{}
	s.syncItemState()
	if !s.items[0].installed {
		t.Fatal("expected search item to be installed after installed origin is present")
	}
}
