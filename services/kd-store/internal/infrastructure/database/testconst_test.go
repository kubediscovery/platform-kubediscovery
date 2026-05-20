package database

// Shared string constants used across table-driven tests in this package.
// Defined here to satisfy the goconst linter and give the values a single
// source of truth throughout the test suite.
const (
	testHost       = "localhost"
	testSSLRequire = "require"
	testUser       = "user"
)
