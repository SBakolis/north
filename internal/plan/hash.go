package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/SBakolis/north/internal/model"
)

// CanonicalJSON normalizes unordered plan collections and returns compact JSON.
func CanonicalJSON(p model.ExecutionPlan) ([]byte, error) {
	ApplyDefaults(&p)
	p.Spec.Stages = append([]model.Stage(nil), p.Spec.Stages...)
	for i := range p.Spec.Stages {
		s := &p.Spec.Stages[i]
		s.DependsOn = append([]string(nil), s.DependsOn...)
		s.WriteScope = append([]string(nil), s.WriteScope...)
		sort.Strings(s.DependsOn)
		sort.Strings(s.WriteScope)
	}
	sort.Slice(p.Spec.Stages, func(i, j int) bool { return p.Spec.Stages[i].ID < p.Spec.Stages[j].ID })
	return json.Marshal(toDocument(p))
}

// ApprovalHash returns the lowercase SHA-256 digest of the canonical plan.
func ApprovalHash(p model.ExecutionPlan) (string, error) {
	ApplyDefaults(&p)
	if err := Validate(p); err != nil {
		return "", err
	}
	data, err := CanonicalJSON(p)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
