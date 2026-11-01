package schemas

import (
	"strings"
)

// Metric inputs for gate evaluation
metric:     "" | "coverage" | "docstring_density" | "intent_traceability_density" | "complexity" | "cds" | "file_loc" | "func_loc" | *""
value:      number & >= 0.0 | *0.0
threshold:  number & >= 0.0 | *0.0
coverage:   number & >= 0.0 & <= 100.0 | *0.0
density:    int & >= 0 & <= 100 | *0
documented: int & >= 0 | *0
total:      int & >= 0 | *0
tagged:     int & >= 0 | *0
missing:    [...string] | *[]
shims:      [...string] | *[]

counts: [string]: [string]: int & >= 0

#FileMetric: close({
	name: string & strings.MinRunes(1)
	loc:  int & >= 0 | *0
	cds:  int & >= 0 | *0
})

files: [...#FileMetric] | *[]

#FunctionMetric: close({
	file:       string & strings.MinRunes(1)
	line:       int & >= 1
	name:       string & strings.MinRunes(1)
	complexity: int & >= 1
	loc:        int & >= 1
})

functions: [...#FunctionMetric] | *[]

#Thresholds: close({
	min_coverage:                 number & >= 0.0 & <= 100.0 | *55.0
	min_docstring_density:        int | *10
	min_traceability_density:     int | *50
	max_cds:                      int | *45
	max_complexity:               int | *15
	max_func_loc:                 int | *60
	max_file_loc:                 int | *400
	max_func_params:              int | *8
	min_clone_lines:              int | *18
	max_magic_string_duplication: int | *2
})

thresholds: #Thresholds

pass: *false | bool

if value >= 0 && (metric == "coverage" || metric == "docstring_density" || metric == "intent_traceability_density") && value >= threshold {
	pass: true
}

if value >= 0 && (metric == "complexity" || metric == "cds" || metric == "file_loc" || metric == "func_loc") && value <= threshold {
	pass: true
}

// Gate Violations Evaluated Declaratively via CUE Unification

coverage_violations: [
	if coverage < thresholds.min_coverage {
		"coverage \(coverage)% failed test execution or is below threshold \(thresholds.min_coverage)%"
	},
]

cds_violations: [
	for f in files if f.cds > thresholds.max_cds {
		"\(f.name) has cognitive debt \(f.cds) (exceeds max \(thresholds.max_cds))"
	},
]

docstring_density_violations: [
	if density < thresholds.min_docstring_density && len(missing) > 0 {
		"docstring density is \(density)% (\(documented)/\(total)), minimum required is \(thresholds.min_docstring_density)%\nMissing:\n    - \(strings.Join(missing, "\n    - "))"
	},
	if density < thresholds.min_docstring_density && len(missing) == 0 {
		"docstring density is \(density)% (\(documented)/\(total)), minimum required is \(thresholds.min_docstring_density)%"
	},
]

magic_string_duplication_violations: [
	for dir, str_counts in counts {
		for str, c in str_counts if c > thresholds.max_magic_string_duplication {
			"Module \(dir): string \"\(str)\" occurs \(c) times"
		}
	},
]

complexity_violations: [
	for fn in functions if fn.complexity > thresholds.max_complexity {
		"\(fn.file):\(fn.line) (\(fn.name)) cyclomatic complexity \(fn.complexity) exceeds max \(thresholds.max_complexity)"
	},
	for fn in functions if fn.loc > thresholds.max_func_loc {
		"\(fn.file):\(fn.line) (\(fn.name)) LOC \(fn.loc) exceeds max \(thresholds.max_func_loc)"
	},
]

file_loc_violations: [
	for f in files if f.loc > thresholds.max_file_loc {
		"\(f.name) has \(f.loc) LOC (exceeds max \(thresholds.max_file_loc))"
	},
]

symlinks: [...string] | *[]

zero_filesystem_alias_violations: [
	if len(symlinks) > 0 {
		"detected \(len(symlinks)) filesystem symlink(s); symlinks are strictly prohibited under Law 6: \(strings.Join(symlinks, "; "))"
	},
]

plane_violations: [...string] | *[]

