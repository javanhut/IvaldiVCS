package butterfly

import (
	"github.com/javanhut/Ivaldi-vcs/internal/cas"
)

type ConflictResolver struct {
	cas cas.CAS
}

func NewConflictResolver(casStore cas.CAS) *ConflictResolver {
	return &ConflictResolver{cas: casStore}
}

func (r *ConflictResolver) FastForwardMerge(oursHash, theirsHash cas.Hash) (cas.Hash, error) {
	if oursHash == theirsHash {
		return oursHash, nil
	}

	return theirsHash, nil
}
