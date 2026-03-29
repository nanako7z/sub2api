// Package tlsfingerprint provides TLS fingerprint simulation for HTTP clients.
package tlsfingerprint

import (
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// DefaultProfileName is the name of the built-in Claude CLI profile.
const DefaultProfileName = "claude_cli_v2"

// Registry manages TLS fingerprint profiles.
// It holds a collection of profiles that can be used for TLS fingerprint simulation.
// Profiles are selected based on account ID using modulo operation.
type Registry struct {
	mu           sync.RWMutex
	profiles     map[string]*Profile
	profileNames []string // Sorted list of profile names for deterministic selection
}

// NewRegistry creates a new TLS fingerprint profile registry.
// It initializes with the built-in default profile.
func NewRegistry() *Registry {
	r := &Registry{
		profiles:     make(map[string]*Profile),
		profileNames: make([]string, 0),
	}

	// Register the built-in default profile
	r.registerBuiltinProfile()

	return r
}

// NewRegistryFromConfig creates a new registry and loads profiles from config.
// If the config has custom profiles defined, they will be merged with the built-in default.
func NewRegistryFromConfig(cfg *config.TLSFingerprintConfig) *Registry {
	r := NewRegistry()

	if cfg == nil || !cfg.Enabled {
		slog.Debug("tls_registry_disabled", "reason", "disabled or no config")
		return r
	}

	if p := r.GetProfile(DefaultProfileName); p != nil {
		// Global profile_mode should control the built-in default profile as well.
		// Per-profile mode overrides are only applicable to custom profiles.
		p.Mode = normalizeProfileMode("", cfg.ProfileMode)
		p.ShapeMode = normalizeShapeMode(cfg.ShapeMode)
		p.ShapeWindowSize = normalizeShapeWindowSize(cfg.ShapeWindowSize)
		p.ShapeWeights = shapeWeightsFromConfig(cfg.ShapeWeights)
		p.ECHPayloadMode = normalizeECHPayloadMode(cfg.ECHPayloadMode)
	}

	// Load custom profiles from config
	for name, profileCfg := range cfg.Profiles {
		profile := &Profile{
			Name:            profileCfg.Name,
			Mode:            normalizeProfileMode(profileCfg.Mode, cfg.ProfileMode),
			ShapeMode:       normalizeShapeMode(cfg.ShapeMode),
			ShapeWindowSize: normalizeShapeWindowSize(cfg.ShapeWindowSize),
			ShapeWeights:    shapeWeightsFromConfig(cfg.ShapeWeights),
			ECHPayloadMode:  normalizeECHPayloadMode(cfg.ECHPayloadMode),
			EnableGREASE:    profileCfg.EnableGREASE,
			CipherSuites:    profileCfg.CipherSuites,
			Curves:          profileCfg.Curves,
			PointFormats:    pointFormatsFromConfig(profileCfg.PointFormats),
		}

		// If the profile has empty values, they will use defaults in dialer
		r.RegisterProfile(name, profile)
		slog.Debug("tls_registry_loaded_profile", "key", name, "name", profileCfg.Name)
	}

	slog.Debug("tls_registry_initialized", "profile_count", len(r.profileNames), "profiles", r.profileNames)
	return r
}

// registerBuiltinProfile adds the default Claude CLI profile to the registry.
func (r *Registry) registerBuiltinProfile() {
	defaultProfile := &Profile{
		Name:            "npm",
		Mode:            "npm",
		ShapeMode:       "npm",
		ShapeWindowSize: 64,
		ShapeWeights:    defaultTLSShapeWeights,
		ECHPayloadMode:  "npm",
		EnableGREASE:    false, // Bun/BoringSSL does not use GREASE
		// Empty slices will cause dialer to use built-in defaults
		CipherSuites: nil,
		Curves:       nil,
		PointFormats: nil,
	}
	r.RegisterProfile(DefaultProfileName, defaultProfile)
}

func normalizeProfileMode(profileMode, globalMode string) string {
	mode := strings.ToLower(strings.TrimSpace(profileMode))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(globalMode))
	}
	if mode == "npm" {
		return mode
	}
	return "npm"
}

func normalizeShapeWindowSize(v int) int {
	if v <= 0 {
		return 64
	}
	return v
}

func shapeWeightsFromConfig(w config.TLSShapeWeightsConfig) TLSShapeWeights {
	out := TLSShapeWeights{
		ECH186Padding41: w.ECH186Padding41,
		ECH218Padding9:  w.ECH218Padding9,
		ECH250Padding0:  w.ECH250Padding0,
		ECH282Padding0:  w.ECH282Padding0,
	}
	if out.ECH186Padding41+out.ECH218Padding9+out.ECH250Padding0+out.ECH282Padding0 <= 0 {
		return defaultTLSShapeWeights
	}
	return out
}

func pointFormatsFromConfig(in []uint16) []uint8 {
	if len(in) == 0 {
		return nil
	}
	out := make([]uint8, 0, len(in))
	for _, v := range in {
		if v > 255 {
			slog.Warn("tls_registry_invalid_point_format", "value", v)
			continue
		}
		out = append(out, uint8(v))
	}
	return out
}

// RegisterProfile adds or updates a profile in the registry.
func (r *Registry) RegisterProfile(name string, profile *Profile) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if this is a new profile
	_, exists := r.profiles[name]
	r.profiles[name] = profile

	if !exists {
		r.profileNames = append(r.profileNames, name)
		// Keep names sorted for deterministic selection
		sort.Strings(r.profileNames)
	}
}

// GetProfile returns a profile by name.
// Returns nil if the profile does not exist.
func (r *Registry) GetProfile(name string) *Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.profiles[name]
}

// GetDefaultProfile returns the built-in default profile.
func (r *Registry) GetDefaultProfile() *Profile {
	return r.GetProfile(DefaultProfileName)
}

// GetProfileByAccountID returns a profile for the given account ID.
// The profile is selected using: profileNames[accountID % len(profiles)]
// This ensures deterministic profile assignment for each account.
func (r *Registry) GetProfileByAccountID(accountID int64) *Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.profileNames) == 0 {
		return nil
	}

	// Use modulo to select profile index
	// Use absolute value to handle negative IDs (though unlikely)
	idx := accountID
	if idx < 0 {
		idx = -idx
	}
	selectedIndex := int(idx % int64(len(r.profileNames)))
	selectedName := r.profileNames[selectedIndex]

	return r.profiles[selectedName]
}

// ProfileCount returns the number of registered profiles.
func (r *Registry) ProfileCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.profiles)
}

// ProfileNames returns a sorted list of all registered profile names.
func (r *Registry) ProfileNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent modification
	names := make([]string, len(r.profileNames))
	copy(names, r.profileNames)
	return names
}

// Global registry instance for convenience
var globalRegistry *Registry
var globalRegistryOnce sync.Once

// GlobalRegistry returns the global TLS fingerprint registry.
// The registry is lazily initialized with the default profile.
func GlobalRegistry() *Registry {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}

// InitGlobalRegistry initializes the global registry with configuration.
// This should be called during application startup.
// It is safe to call multiple times; subsequent calls will update the registry.
func InitGlobalRegistry(cfg *config.TLSFingerprintConfig) *Registry {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewRegistryFromConfig(cfg)
	})
	return globalRegistry
}
