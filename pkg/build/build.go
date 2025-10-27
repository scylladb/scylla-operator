package build

// commitFromGit is a constant representing the source version that
// generated this build. It should be set during build via -ldflags.
var commitFromGit string

// GitCommit returns the git commit hash for the source used to build the binary.
func GitCommit() string {
	return commitFromGit
}
