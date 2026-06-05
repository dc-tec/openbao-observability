package contracts

import "fmt"

type Maturity struct {
	Lifecycle string   `yaml:"lifecycle"`
	Evidence  []string `yaml:"evidence"`
}

const (
	maturityLifecycleDraft      = "draft"
	maturityLifecycleReference  = "reference"
	maturityLifecycleStable     = "stable"
	maturityLifecycleDeprecated = "deprecated"

	maturityEvidenceDocumented          = "documented"
	maturityEvidenceFixtureValidated    = "fixture-validated"
	maturityEvidenceGeneratedValidated  = "generated-validated"
	maturityEvidenceProfileValidated    = "profile-validated"
	maturityEvidenceExternallyValidated = "externally-validated"
)

var (
	allowedMaturityLifecycles = stringSet([]string{
		maturityLifecycleDraft,
		maturityLifecycleReference,
		maturityLifecycleStable,
		maturityLifecycleDeprecated,
	})
	allowedMaturityEvidence = stringSet([]string{
		maturityEvidenceDocumented,
		maturityEvidenceFixtureValidated,
		maturityEvidenceGeneratedValidated,
		maturityEvidenceProfileValidated,
		maturityEvidenceExternallyValidated,
	})
)

func validateMaturity(path string, maturity Maturity) error {
	if maturity.Lifecycle == "" {
		return fmt.Errorf("contract %s is missing maturity.lifecycle", path)
	}
	if !allowedMaturityLifecycles[maturity.Lifecycle] {
		return fmt.Errorf("contract %s has unsupported maturity.lifecycle %q", path, maturity.Lifecycle)
	}
	if len(maturity.Evidence) == 0 {
		return fmt.Errorf("contract %s is missing maturity.evidence", path)
	}

	seen := map[string]bool{}
	for _, evidence := range maturity.Evidence {
		if evidence == "" {
			return fmt.Errorf("contract %s has empty maturity.evidence entry", path)
		}
		if !allowedMaturityEvidence[evidence] {
			return fmt.Errorf("contract %s has unsupported maturity.evidence %q", path, evidence)
		}
		if seen[evidence] {
			return fmt.Errorf("contract %s has duplicate maturity.evidence %q", path, evidence)
		}
		seen[evidence] = true
	}

	return nil
}
