package submodule

import (
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	bolt "go.etcd.io/bbolt"
)

type Submodule struct {
	Name      string
	Path      string
	URL       string
	Timeline  string
	Commit    cas.Hash
	GitCommit string
	Shallow   bool
	Freeze    bool
}

type TimelineSubmoduleState struct {
	TimelineName  string
	SubmodulePath string
	CommitHash    cas.Hash
	GitCommitSHA1 string
	LocalTimeline string
	Modified      bool
	LastUpdate    time.Time
}

type Manager struct {
	IvaldiDir string
	WorkDir   string
	DB        *bolt.DB
}

type SubmoduleStatus struct {
	Path           string
	CurrentCommit  cas.Hash
	ExpectedCommit cas.Hash
	HasChanges     bool
	Timeline       string
	NeedsUpdate    bool
}

type Config struct {
	Name      string
	Path      string
	URL       string
	Timeline  string
	Commit    string
	GitCommit string
	Shallow   bool
	Freeze    bool
	Ignore    string
}
