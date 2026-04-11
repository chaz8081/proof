package review

import "github.com/chaz8081/proof/internal/config"

// builtinProfiles contains the built-in review profiles shipped with proof.
var builtinProfiles = map[string]config.ReviewProfile{
	"quick": {
		Instructions: "Focus only on bugs, security issues, and blockers. Skip style nits and minor suggestions. Be brief — max 3-5 comments on the most critical issues only.",
		SeverityMin:  "issue",
		MaxComments:  5,
	},
	"thorough": {
		Instructions: "Do a comprehensive review. Check for bugs, security, performance, readability, test coverage, error handling, naming, and style. Be detailed and include suggestions with code examples where possible.",
		SeverityMin:  "nit",
		MaxComments:  0, // no limit
	},
}

// ResolveProfile looks up a profile by name, checking built-ins first then
// user-defined profiles in config. Returns nil when the name is not found.
func ResolveProfile(name string, cfg *config.Config) *config.ReviewProfile {
	// Check built-in first
	if p, ok := builtinProfiles[name]; ok {
		return &p
	}
	// Check user-defined profiles in config
	if cfg != nil && cfg.Review.Profiles != nil {
		if p, ok := cfg.Review.Profiles[name]; ok {
			return &p
		}
	}
	return nil
}
