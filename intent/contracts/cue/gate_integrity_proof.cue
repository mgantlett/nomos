package schemas

// GateIntegrityReport defines the machine-enforced contract for deterministic gate sensitivity, specificity, and non-vacuity attestation.
#GateIntegrityReport: {
	target_dir:             string
	is_codebase_targeted:   bool
	codebase_files_scanned: int & >=0
	mutation_killed:        int & >=0
	mutation_total:         int & >=0
	mutation_score:         float & >=0.0 & <=100.0
	false_positive_rate:    float & >=0.0 & <=100.0
	reference_gates_pass:   int & >=0
	reference_gates_total:  int & >0
	non_vacuous_gates:      int & >=0
	total_gates_evaluated:  int & >0
	passed:                 bool

	// Invariant Bounds (Cell 2.3)
	// 1. Mutation Sensitivity: 100% of tested failure mutants must be killed
	_sensitivity_pass: mutation_score >= 100.0
	// 2. Metamorphic Specificity: zero false positive alarms on compliant constructs
	_specificity_pass: false_positive_rate == 0.0
	// 3. Structural Anti-Vacuity: 100% of discovered gate Evaluate functions must be non-vacuous
	_anti_vacuous_pass: non_vacuous_gates == total_gates_evaluated

	// Conjunction: passed strictly reflects the conjunction of all three invariant attestation vectors
	passed: _sensitivity_pass && _specificity_pass && _anti_vacuous_pass
}

// Enforce closed-world shape
#GateIntegrityReport
