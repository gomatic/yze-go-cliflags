package cliflags_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	urf "github.com/urfave/cli/v3"
)

// TestCheckMetaSourcesEnvBindingIsInertUpstream pins the upstream fact behind the meta-flag
// rule: urfave v3 applies Sources in FlagBase.PostParse, after its version
// check, so an env-bound VersionFlag override never triggers version output —
// the run falls through to the action as if the variable were unset. If this
// test ever fails, upstream reordered the check and the rule must be
// reconsidered.
func TestCheckMetaSourcesEnvBindingIsInertUpstream(t *testing.T) {
	orig := urf.VersionFlag
	t.Cleanup(func() { urf.VersionFlag = orig })
	t.Setenv("CLIFLAGS_META_PROBE_VERSION", "true")
	urf.VersionFlag = &urf.BoolFlag{
		Name:    "version",
		Usage:   "print version information and exit",
		Sources: urf.EnvVars("CLIFLAGS_META_PROBE_VERSION"),
	}

	ran := false
	var out bytes.Buffer
	cmd := &urf.Command{
		Name:    "probe",
		Version: "9.9.9",
		Writer:  &out,
		Action:  func(context.Context, *urf.Command) error { ran = true; return nil },
	}
	require.NoError(t, cmd.Run(context.Background(), []string{"probe"}))

	assert.True(t, ran, "the action must run: the env-bound version flag is inert upstream")
	assert.Empty(t, out.String(), "no version output despite the env var being set")
}

// TestCheckMetaSourcesCompletionBindingIsInertUpstream pins the upstream fact
// behind treating cli.GenerateShellCompletionFlag as a meta-flag: v3 decides
// completion from the LITERAL last argument "--generate-shell-completion"
// (completion.go:14, help.go:482) and reads the variable nowhere, so an env
// binding on an override of it cannot ever trigger completion. If this test
// ever fails, upstream started reading the variable and the rule must be
// reconsidered.
func TestCheckMetaSourcesCompletionBindingIsInertUpstream(t *testing.T) {
	orig := urf.GenerateShellCompletionFlag
	t.Cleanup(func() { urf.GenerateShellCompletionFlag = orig })
	t.Setenv("CLIFLAGS_META_PROBE_COMPLETION", "true")
	urf.GenerateShellCompletionFlag = &urf.BoolFlag{
		Name:    "generate-shell-completion",
		Sources: urf.EnvVars("CLIFLAGS_META_PROBE_COMPLETION"),
	}

	ran := false
	var out bytes.Buffer
	cmd := &urf.Command{
		Name:                  "probe",
		EnableShellCompletion: true,
		Writer:                &out,
		Commands:              []*urf.Command{{Name: "sub"}},
		Action:                func(context.Context, *urf.Command) error { ran = true; return nil },
	}
	require.NoError(t, cmd.Run(context.Background(), []string{"probe"}))

	assert.True(t, ran, "the action must run: the env-bound completion flag is inert upstream")
	assert.Empty(t, out.String(), "no completion listing despite the env var being set")
}
