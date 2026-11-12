// Package impact infers zero-violation directional architecture import boundaries
// across existing polyglot brownfield repositories during nomos init (Law 4 & Law 7).
package impact

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

type layerRole int

const (
	roleNone layerRole = iota
	roleDomain
	roleApp
	roleInfra
	roleDelivery
	maxInferredBoundaryRules = 6
	maxInferenceDirDepth     = 4
)

var (
	domainLayerNames = map[string]bool{
		"domain": true, "core": true, "model": true,
		"models": true, "entity": true, "entities": true,
	}
	appLayerNames = map[string]bool{
		"app": true, "application": true, "usecase": true,
		"usecases": true, "service": true, "services": true,
	}
	infraLayerNames = map[string]bool{
		"infra": true, "infrastructure": true, "adapter": true,
		"adapters": true, "db": true, "persistence": true,
		"repository": true, "repositories": true,
	}
	deliveryLayerNames = map[string]bool{
		"api": true, "cli": true, "cmd": true, "routes": true,
		"ui": true, "web": true, "handlers": true, "controllers": true,
		"transport": true, "grpc": true, "http": true,
	}
)

type classifiedLayerDir struct {
	relDir string
	role   layerRole
}

// InferCleanArchitectureBoundaries discovers directional layer import rules that satisfy
// existing repository imports with zero baseline violations (@intent nomos_intent_traceability_spec.md #ArchitectureBoundaryRule).
func InferCleanArchitectureBoundaries(root string) []config.ArchitectureBoundaryRule {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	allFiles := listNonTestBoundarySourceFiles(root)
	if len(allFiles) == 0 {
		return nil
	}
	layers := discoverClassifiedLayerDirs(allFiles)
	if len(layers) < 2 {
		return nil
	}
	return synthesizeVerifiedBoundaryRules(root, allFiles, layers)
}

func listNonTestBoundarySourceFiles(root string) []string {
	var out []string
	walkErr := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if p != root && (strings.HasPrefix(info.Name(), ".") || IsSkippedObligationDir(info.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if !vscope.IsSupportedPolyglotSourceFile(p) || IsVerificationSuiteFile(info.Name()) {
			return nil
		}
		if rel, rErr := filepath.Rel(root, p); rErr == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if walkErr != nil {
		return out
	}
	sort.Strings(out)
	return out
}

func discoverClassifiedLayerDirs(files []string) []classifiedLayerDir {
	roleByDir := make(map[string]layerRole)
	for _, relFile := range files {
		slashDir := filepath.ToSlash(filepath.Dir(relFile))
		if slashDir == "." || slashDir == "" {
			continue
		}
		recordLayerAncestors(slashDir, roleByDir)
	}
	pruneContainerAppLayers(roleByDir)
	var out []classifiedLayerDir
	for dir, role := range roleByDir {
		out = append(out, classifiedLayerDir{relDir: dir, role: role})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].relDir < out[j].relDir })
	return out
}

func recordLayerAncestors(slashDir string, roleByDir map[string]layerRole) {
	segs := strings.Split(slashDir, "/")
	limit := len(segs)
	if limit > maxInferenceDirDepth {
		limit = maxInferenceDirDepth
	}
	for i := 0; i < limit; i++ {
		prefix := strings.Join(segs[:i+1], "/")
		if _, exists := roleByDir[prefix]; exists {
			continue
		}
		if r := classifySegmentRole(segs[i]); r != roleNone {
			roleByDir[prefix] = r
		}
	}
}

func classifySegmentRole(seg string) layerRole {
	lower := strings.ToLower(strings.TrimSpace(seg))
	switch {
	case domainLayerNames[lower]:
		return roleDomain
	case appLayerNames[lower]:
		return roleApp
	case infraLayerNames[lower]:
		return roleInfra
	case deliveryLayerNames[lower]:
		return roleDelivery
	default:
		return roleNone
	}
}

// pruneContainerAppLayers removes outer container prefixes (such as "app/") when nested
// architectural layers (such as "app/domain" or "app/infra") live underneath them.
func pruneContainerAppLayers(roleByDir map[string]layerRole) {
	for candidate, role := range roleByDir {
		if role != roleApp {
			continue
		}
		for other := range roleByDir {
			if other != candidate && strings.HasPrefix(other, candidate+"/") {
				delete(roleByDir, candidate)
				break
			}
		}
	}
}

func synthesizeVerifiedBoundaryRules(root string, allFiles []string, layers []classifiedLayerDir) []config.ArchitectureBoundaryRule {
	var rules []config.ArchitectureBoundaryRule
	for _, inner := range layers {
		if inner.role != roleDomain && inner.role != roleApp {
			continue
		}
		forbids := collectZeroViolationForbids(root, allFiles, inner, layers)
		if len(forbids) == 0 {
			continue
		}
		rules = append(rules, config.ArchitectureBoundaryRule{
			Name:          deriveBoundaryRuleName(inner),
			From:          inner.relDir + "/**",
			ForbidImports: forbids,
		})
		if len(rules) >= maxInferredBoundaryRules {
			break
		}
	}
	return rules
}

func collectZeroViolationForbids(root string, allFiles []string, inner classifiedLayerDir, layers []classifiedLayerDir) []string {
	var forbids []string
	for _, outer := range layers {
		if !isCandidateOuterForInner(inner, outer) {
			continue
		}
		glob := outer.relDir + "/**"
		probe := []config.ArchitectureBoundaryRule{{
			Name:          "probe",
			From:          inner.relDir + "/**",
			ForbidImports: []string{glob},
		}}
		if len(EvaluateArchitectureBoundaries(root, allFiles, probe)) == 0 {
			forbids = append(forbids, glob)
		}
	}
	return forbids
}

func isCandidateOuterForInner(inner, outer classifiedLayerDir) bool {
	if inner.relDir == outer.relDir {
		return false
	}
	if strings.HasPrefix(inner.relDir, outer.relDir+"/") || strings.HasPrefix(outer.relDir, inner.relDir+"/") {
		return false
	}
	if inner.role == roleDomain {
		return outer.role == roleInfra || outer.role == roleDelivery
	}
	if inner.role == roleApp {
		return outer.role == roleDelivery
	}
	return false
}

func deriveBoundaryRuleName(inner classifiedLayerDir) string {
	slug := strings.ReplaceAll(strings.ToLower(inner.relDir), "/", "-")
	if inner.role == roleDomain {
		return slug + "-purity"
	}
	return slug + "-isolation"
}
