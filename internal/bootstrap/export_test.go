package bootstrap

// Exports for external (bootstrap_test) tests.
func Sha256MatchesForTest(path, expected string) bool { return sha256Matches(path, expected) }
