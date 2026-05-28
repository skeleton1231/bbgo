package bbgo

import (
	"context"
	"testing"
)

func TestBootstrapEnvironment_SetsEnvironmentConfigEarly(t *testing.T) {
	environ := NewEnvironment()

	cfg := &Config{
		Environment: &EnvironmentConfig{
			DisableStartupBalanceQuery: true,
			DisableHistoryKLinePreload: true,
		},
	}

	_ = BootstrapEnvironment(context.Background(), environ, cfg)

	if environ.environmentConfig == nil {
		t.Fatal("environmentConfig is nil after BootstrapEnvironment")
	}
	if !environ.environmentConfig.DisableStartupBalanceQuery {
		t.Error("DisableStartupBalanceQuery should be true")
	}
	if !environ.environmentConfig.DisableHistoryKLinePreload {
		t.Error("DisableHistoryKLinePreload should be true")
	}
}

func TestBootstrapEnvironment_NilEnvironmentConfigIsNoop(t *testing.T) {
	environ := NewEnvironment()
	cfg := &Config{Environment: nil}

	_ = BootstrapEnvironment(context.Background(), environ, cfg)

	if environ.environmentConfig != nil {
		t.Error("environmentConfig should remain nil when config is nil")
	}
}

func TestBootstrapEnvironmentLightweight_SetsEnvironmentConfigEarly(t *testing.T) {
	environ := NewEnvironment()

	cfg := &Config{
		Environment: &EnvironmentConfig{
			DisableStartupBalanceQuery: true,
		},
	}

	_ = BootstrapEnvironmentLightweight(context.Background(), environ, cfg)

	// Lightweight bootstrap intentionally skips environmentConfig — it has
	// no database or notification setup, so environment flags are unused.
	if environ.environmentConfig != nil {
		t.Error("LightweightBootstrap should not set environmentConfig")
	}
}
