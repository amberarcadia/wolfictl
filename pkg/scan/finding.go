package scan

import (
	"fmt"
	"strings"

	"github.com/anchore/grype/grype/match"
	"github.com/anchore/grype/grype/vulnerability"
	"github.com/anchore/syft/syft/file"
	"github.com/samber/lo"
	v2 "github.com/wolfi-dev/wolfictl/pkg/configs/advisory/v2"
)

// Finding represents a vulnerability finding for a single package.
type Finding struct {
	Package       Package
	Vulnerability Vulnerability
	Advisory      *v2.Advisory `json:",omitempty"`

	// Deprecated: This field will be removed soon.
	TriageAssessments []TriageAssessment `json:",omitempty"`
}

type Findings []Finding

func (f Findings) Len() int {
	return len(f)
}

func (f Findings) Less(i, j int) bool {
	fi := f[i]
	fj := f[j]

	if fi.Package.Location != fj.Package.Location {
		return fi.Package.Location < fj.Package.Location
	}

	if fi.Package.Name != fj.Package.Name {
		return fi.Package.Name < fj.Package.Name
	}

	if fi.Vulnerability.ID != fj.Vulnerability.ID {
		return fi.Vulnerability.ID < fj.Vulnerability.ID
	}

	return true
}

func (f Findings) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

type Package struct {
	ID       string
	Name     string
	Version  string
	Type     string
	Location string
	PURL     string
}

type Vulnerability struct {
	ID           string
	Severity     string
	Aliases      []string
	FixedVersion string
}

// Deprecated: This type will be removed soon.
type TriageAssessment struct {
	// Source is the name of the source of the triage assessment, e.g.
	// "govulncheck".
	Source string

	// TruePositive indicates whether the vulnerability is a true positive. A value
	// of false indicates that the vulnerability has been assessed to be a false
	// positive.
	TruePositive bool

	// Reason is the explanation of the triage assessment.
	Reason string
}

func mapMatchToFinding(m match.Match, vulnProvider vulnerability.Provider) (*Finding, error) {
	metadata, err := vulnProvider.VulnerabilityMetadata(m.Vulnerability.Reference)
	if err != nil {
		return nil, fmt.Errorf("retrieving metadata for vulnerability %s (%s): %w", m.Vulnerability.ID, m.Vulnerability.Namespace, err)
	}

	var relatedMetadatas []*vulnerability.Metadata
	for _, relatedRef := range m.Vulnerability.RelatedVulnerabilities {
		relatedMetadata, err := vulnProvider.VulnerabilityMetadata(relatedRef)
		if err != nil {
			return nil, fmt.Errorf("retrieving metadata for related vulnerability %s (%s): %w", relatedRef.ID, relatedRef.Namespace, err)
		}
		if relatedMetadata == nil {
			continue
		}
		relatedMetadatas = append(relatedMetadatas, relatedMetadata)
	}

	aliases := lo.Map(relatedMetadatas, func(m *vulnerability.Metadata, _ int) string {
		return m.ID
	})

	locations := lo.Map(m.Package.Locations.ToSlice(), func(l file.Location, _ int) string {
		return "/" + l.RealPath
	})

	f := &Finding{
		Package: Package{
			ID:       string(m.Package.ID),
			Name:     m.Package.Name,
			Version:  m.Package.Version,
			Type:     string(m.Package.Type),
			Location: strings.Join(locations, ", "),
			PURL:     m.Package.PURL,
		},
		Vulnerability: Vulnerability{
			ID:           m.Vulnerability.ID,
			Severity:     metadata.Severity,
			Aliases:      aliases,
			FixedVersion: getFixedVersion(m.Vulnerability),
		},
	}

	return f, nil
}

func getFixedVersion(vuln vulnerability.Vulnerability) string {
	if vuln.Fix.State != vulnerability.FixStateFixed {
		return ""
	}

	return strings.Join(vuln.Fix.Versions, ", ")
}
