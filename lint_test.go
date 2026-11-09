package gorepo

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/spf13/viper"
)

// readConfig decodes what writeConfig wrote as golangci-lint does, into a config that holds only what was written.
func readConfig(t *testing.T, linters config.Linters) config.Linters {
	t.Helper()

	var b bytes.Buffer
	if err := writeConfig(&b, linters); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	v.SetConfigType("json")

	if err := v.ReadConfig(bytes.NewReader(b.Bytes())); err != nil {
		t.Fatalf("not a config golangci-lint can read: %v\n%s", err, b.Bytes())
	}

	var cfg config.Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(config.DecodeHookFunc())); err != nil {
		t.Fatalf("golangci-lint cannot decode it: %v\n%s", err, b.Bytes())
	}

	if cfg.Version != "2" {
		t.Errorf("version %q, want 2", cfg.Version)
	}

	return cfg.Linters
}

// What the caller sets is written; what it leaves at zero keeps golangci-lint's default, rather than turning a default
// of true off as golangci-json would write a zero.
func TestWriteConfig_KeepsDefaults(t *testing.T) {
	t.Parallel()

	got := readConfig(t, config.Linters{
		Default: config.GroupNone,
		Enable:  []string{"godot", "usetesting"},
		Settings: config.LintersSettings{
			Godot:      config.GodotSettings{Scope: "all"},
			UseTesting: config.UseTestingSettings{OSCreateTemp: true},
			Decorder:   config.DecorderSettings{DecOrder: []string{"func", "type"}},
		},
	})

	if got.Default != config.GroupNone || !reflect.DeepEqual(got.Enable, []string{"godot", "usetesting"}) {
		t.Errorf("default %q, enable %v: want none, godot and usetesting", got.Default, got.Enable)
	}

	def := config.NewDefault().Linters.Settings

	if got.Settings.Godot.Scope != "all" {
		t.Errorf("godot scope %q, want the caller's all", got.Settings.Godot.Scope)
	}

	if got.Settings.Godot.Period != def.Godot.Period || !got.Settings.Godot.Period {
		t.Errorf("godot period %t, want golangci-lint's default, true", got.Settings.Godot.Period)
	}

	if !got.Settings.UseTesting.OSCreateTemp || got.Settings.UseTesting.OSChdir != def.UseTesting.OSChdir {
		t.Errorf("usetesting: %+v, want os-create-temp set and the rest at golangci-lint's defaults", got.Settings.UseTesting)
	}

	if !reflect.DeepEqual(got.Settings.Decorder.DecOrder, []string{"func", "type"}) {
		t.Errorf("decorder dec-order %v, want the caller's", got.Settings.Decorder.DecOrder)
	}
}

// An exclusion rule is written with golangci-lint's own keys, so it applies.
func TestWriteConfig_ExclusionRules(t *testing.T) {
	t.Parallel()

	rule := config.ExcludeRule{Path: `_test\.go`, Linters: []string{"godot"}, Text: "period"}

	got := readConfig(t, config.Linters{
		Default:    config.GroupNone,
		Enable:     []string{"godot"},
		Exclusions: config.LinterExclusions{Rules: []config.ExcludeRule{rule}},
	})

	if !reflect.DeepEqual(got.Exclusions.Rules, []config.ExcludeRule{rule}) {
		t.Errorf("exclusion rules read back as %+v, want %+v", got.Exclusions.Rules, rule)
	}
}
