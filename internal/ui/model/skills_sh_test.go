package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSkillsSHSourcesOutput(t *testing.T) {
	t.Parallel()

	t.Run("empty output", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, parseSkillsSHSourcesOutput(""))
	})

	t.Run("no tracked sources output", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, parseSkillsSHSourcesOutput("No tracked skills.sh sources."))
	})

	t.Run("source list output", func(t *testing.T) {
		t.Parallel()
		output := "skills.sh/owner/repo\nskills.sh/owner/repo/skill-a\n"
		require.Equal(t, []string{
			"skills.sh/owner/repo",
			"skills.sh/owner/repo/skill-a",
		}, parseSkillsSHSourcesOutput(output))
	})
}

func TestBuildSkillsSHInstallSource(t *testing.T) {
	t.Parallel()

	require.Equal(t, "skills.sh/owner/repo/slug-name",
		buildSkillsSHInstallSource("owner/repo", "slug-name", "", ""))
	require.Equal(t, "skills.sh/owner/repo/human-name",
		buildSkillsSHInstallSource("owner/repo", "", "opaque-id", "human-name"))
	require.Equal(t, "skills.sh/owner/repo/skill-id",
		buildSkillsSHInstallSource("owner/repo", "", "skill-id", ""))
	require.Equal(t, "skills.sh/owner/repo",
		buildSkillsSHInstallSource("owner/repo", "", "", ""))
	require.Equal(t, "",
		buildSkillsSHInstallSource("", "slug", "id", "name"))
}
