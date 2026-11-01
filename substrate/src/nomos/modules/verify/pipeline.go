// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// Sensor constants define the available telemetry drivers (#SensorType).
const (
	SensorGit          = "git"
	SensorAST          = "ast"
	SensorTestCoverage = "test_coverage"
	SensorSecret       = "secret"
	SensorLinks        = "links"
	SensorTraceability = "traceability"
	SensorCustom       = "custom"
)

// GateSpec defines the declarative schema for an individual sensor probe (#GateSpec).
type GateSpec struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Law     int    `json:"law"`
	Ring    string `json:"ring"`
	Domain  string `json:"domain"`
	Sensor  string `json:"sensor"`
	Phase   string `json:"phase"`
	Enabled bool   `json:"enabled"`
	Remedy  string `json:"remedy"`
	RuleKey string `json:"rule_key"`
}

// PipelineDeclaration models the compiled pipeline schema from CUE (#Pipeline).
type PipelineDeclaration struct {
	Version string              `json:"version"`
	Cells   []InvariantCellSpec `json:"cells,omitempty"`
	Gates   []GateSpec          `json:"gates"`
}

// PipelineReport encapsulates the complete verification results across all gates, cells, and laws.
type PipelineReport struct {
	Passed         bool          `json:"passed"`
	DurationMicros int64         `json:"duration_micros"`
	GatesTotal     int           `json:"gates_total"`
	GatesPassed    int           `json:"gates_passed"`
	GateResults    []*GateResult `json:"gate_results"`
	LawReports     []*LawReport  `json:"law_reports"`
	CellReports    []*CellReport `json:"cell_reports"`
}

// Pipeline executes an ordered suite of Definition of Done quality gates.
type Pipeline struct {
	gates []Gate
}

// NewPipeline creates a new verification pipeline instance with the provided gates.
func NewPipeline(gates []Gate) *Pipeline {
	return &Pipeline{gates: gates}
}

// Execute runs all configured gates sequentially, aggregates results, and builds matrix reports.
func (p *Pipeline) Execute(ctx *VerifyContext) (*PipelineReport, error) {
	if ctx == nil {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "verify context cannot be nil")
	}

	start := time.Now()
	results := make([]*GateResult, 0, len(p.gates))
	overallPassed := true

	for _, g := range p.gates {
		res, err := TimedEvaluate(g, ctx)
		if err != nil {
			return nil, types.NewNomosError(types.ErrCodeCore, "gate %s evaluation failed: %v", g.Name(), err)
		}
		if res == nil {
			res = &GateResult{GateName: g.Name(), Passed: false, Message: "nil gate result returned"}
		}
		if !res.Passed && !res.Skipped {
			overallPassed = false
		}
		results = append(results, res)
	}

	elapsed := time.Since(start).Microseconds()
	cellReports := buildCellReports(results)
	lawReports := buildLawReports(cellReports)

	return &PipelineReport{
		Passed:         overallPassed,
		DurationMicros: elapsed,
		GatesTotal:     len(results),
		GatesPassed:    countPassed(results),
		GateResults:    results,
		LawReports:     lawReports,
		CellReports:    cellReports,
	}, nil
}

func countPassed(results []*GateResult) int {
	passed := 0
	for _, r := range results {
		if r.Passed || r.Skipped {
			passed++
		}
	}
	return passed
}

// buildCellReports maps gate results into the 21 canonical invariant cells.
func buildCellReports(results []*GateResult) []*CellReport {
	resMap := make(map[string]*GateResult, len(results))
	for _, r := range results {
		if r.ProbeID != "" {
			resMap[r.ProbeID] = r
		}
		resMap[r.GateName] = r
	}

	cells := DefaultBaselineCells()
	reports := make([]*CellReport, 0, len(cells))

	for _, cell := range cells {
		cr := aggregateCell(cell, resMap)
		reports = append(reports, cr)
	}
	return reports
}

func aggregateCell(cell InvariantCellSpec, resMap map[string]*GateResult) *CellReport {
	cr := &CellReport{
		ID:         cell.ID,
		Name:       cell.Name,
		Law:        cell.Law,
		Ring:       cell.Ring,
		Domain:     cell.Domain,
		Passed:     true,
		Remedy:     cell.Remedy,
		ProbeCount: len(cell.ProbeIDs),
		Probes:     make([]*GateResult, 0, len(cell.ProbeIDs)),
	}

	for _, pid := range cell.ProbeIDs {
		if r, ok := resMap[pid]; ok {
			cr.Probes = append(cr.Probes, r)
			cr.DurationMicros += r.DurationMicros
			if r.Passed || r.Skipped {
				cr.PassedProbes++
			} else {
				cr.Passed = false
			}
		}
	}
	return cr
}

// buildLawReports aggregates cell reports into 7 statutory law reports.
func buildLawReports(cells []*CellReport) []*LawReport {
	laws := make([]*LawReport, 7)
	for i, l := range CanonicalLaws {
		laws[i] = &LawReport{
			Law:    l.Law,
			Name:   l.Name,
			Domain: l.Domain,
			Passed: true,
			Cells:  make([]*CellReport, 0, 3),
		}
	}

	for _, c := range cells {
		idx := c.Law - 1
		if idx >= 0 && idx < 7 {
			laws[idx].TotalCells++
			laws[idx].DurationMicros += c.DurationMicros
			laws[idx].Cells = append(laws[idx].Cells, c)
			if c.Passed {
				laws[idx].PassedCells++
			} else {
				laws[idx].Passed = false
			}
		}
	}
	return laws
}
