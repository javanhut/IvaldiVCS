package butterfly

import (
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
)

type Butterfly struct {
	Name           string
	ParentName     string
	DivergenceHash cas.Hash
	CreatedAt      time.Time
	IsOrphaned     bool
	OriginalParent string
}

type ButterflyMetadata struct {
	Timeline    string
	IsButterfly bool
	Butterfly   *Butterfly
	Children    []string
}
