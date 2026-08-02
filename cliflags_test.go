package cliflags_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	cliflags "github.com/gomatic/yze-go-cliflags"
)

// TestFlagStandards pins every rule against the fixtures: conformant flags are
// silent, and each violation — missing Sources, a Sources that binds no
// environment variable, a missing Value on a non-boolean, a non-kebab name, a
// Required flag, a prefixed well-known external variable, and a
// non-UPPERCASE_SNAKE name (including one reached through a named constant) —
// is reported by its rule.
func TestFlagStandards(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), cliflags.Analyzer, "app")
}

// TestAppPrefixSetting pins -app: app-specific variables must carry the
// UPPERCASE_SNAKE app prefix (my-kilroy becomes MY_KILROY_), while well-known
// external variables stay exempt.
func TestAppPrefixSetting(t *testing.T) {
	require.NoError(t, cliflags.Analyzer.Flags.Set("app", "my-kilroy"))
	t.Cleanup(func() { _ = cliflags.Analyzer.Flags.Set("app", "") })

	analysistest.Run(t, analysistest.TestData(), cliflags.Analyzer, "appprefix")
}

// TestExternalSetting pins -external: an added namespace is honored unprefixed
// and reported when wrapped under an app prefix, exactly like the defaults.
func TestExternalSetting(t *testing.T) {
	require.NoError(t, cliflags.Analyzer.Flags.Set("external", "VAULT_"))
	t.Cleanup(func() { _ = cliflags.Analyzer.Flags.Set("external", "") })

	analysistest.Run(t, analysistest.TestData(), cliflags.Analyzer, "external")
}

// TestRegistrationIsWellFormed pins the yze wiring.
func TestRegistrationIsWellFormed(t *testing.T) {
	assert.NoError(t, cliflags.Registration.Validate())
	assert.Equal(t, "yze/cliflags", cliflags.Registration.RuleID())
	assert.Same(t, cliflags.Analyzer, cliflags.Registration.Analyzer)
}
