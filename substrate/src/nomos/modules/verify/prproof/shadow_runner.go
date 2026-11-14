// Package prproof executes shadow-workspace Go test runs and tar archive extraction.
package prproof

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func executeShadowMutants(absTarget, baseRef string, sites []rawMutantSite, started time.Time) (*DiffMutationReport, error) {
	grouped := make(map[string][]rawMutantSite)
	for _, s := range sites {
		grouped[s.modRoot] = append(grouped[s.modRoot], s)
	}
	rep := &DiffMutationReport{BaseRef: baseRef}
	for modRoot, modSites := range grouped {
		if err := evaluateModuleShadowMutants(absTarget, modRoot, modSites, rep); err != nil {
			return nil, err
		}
	}
	rep.MutantsSurvived = rep.MutantsTotal - rep.MutantsKilled
	if rep.MutantsTotal == 0 {
		rep.KillScorePct = 100.0
	} else {
		rep.KillScorePct = float64(rep.MutantsKilled) * 100.0 / float64(rep.MutantsTotal)
	}
	rep.Passed = rep.MutantsSurvived == 0
	rep.DurationMillis = time.Since(started).Milliseconds()
	return rep, nil
}

func evaluateModuleShadowMutants(absTarget, modRoot string, sites []rawMutantSite, rep *DiffMutationReport) error {
	shadowDir, err := os.MkdirTemp("", "nomos-mut-shadow-*")
	if err != nil {
		return fmt.Errorf("failed creating shadow workspace: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(shadowDir); rmErr != nil && !os.IsNotExist(rmErr) {
			return
		}
	}()
	if cErr := copyGoModuleToShadow(modRoot, shadowDir); cErr != nil {
		return fmt.Errorf("failed copying shadow workspace (%s): %w", filepath.Base(absTarget), cErr)
	}
	verifiedPkgs := make(map[string]bool)
	for _, site := range sites {
		if !verifiedPkgs[site.pkgRel] {
			if !runShadowGoTestPasses(shadowDir, site.pkgRel) {
				return fmt.Errorf("baseline tests failed in %s before mutation probing", site.pkgRel)
			}
			verifiedPkgs[site.pkgRel] = true
		}
		killed, pErr := probeSingleShadowMutant(shadowDir, site)
		if pErr != nil {
			return pErr
		}
		rep.MutantsTotal++
		site.meta.Killed = killed
		if killed {
			rep.MutantsKilled++
		} else {
			rep.Survivors = append(rep.Survivors, site.meta)
		}
	}
	return nil
}

func probeSingleShadowMutant(shadowDir string, site rawMutantSite) (bool, error) {
	targetFile := filepath.Join(shadowDir, filepath.FromSlash(site.fileRel))
	orig, err := os.ReadFile(targetFile)
	if err != nil {
		return false, err
	}
	mutated := make([]byte, 0, len(orig)+4)
	mutated = append(mutated, orig[:site.offset]...)
	mutated = append(mutated, []byte(site.meta.MutatedOp)...)
	mutated = append(mutated, orig[site.offset+site.origBytes:]...)
	if wErr := os.WriteFile(targetFile, mutated, 0600); wErr != nil {
		return false, wErr
	}
	killed := !runShadowGoTestPasses(shadowDir, site.pkgRel)
	if rErr := os.WriteFile(targetFile, orig, 0600); rErr != nil {
		return killed, rErr
	}
	return killed, nil
}

func runShadowGoTestPasses(shadowDir, pkgRel string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), mutantTestTimeoutS*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", pkgRel)
	cmd.Dir = shadowDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd.Run() == nil
}

func isSkippedShadowDir(name string) bool {
	switch name {
	case ".git", "worktrees", "node_modules", ".tmp", "dist", "build":
		return true
	default:
		return false
	}
}

func copyGoModuleToShadow(srcModRoot, dstShadow string) error {
	return filepath.WalkDir(srcModRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, rErr := filepath.Rel(srcModRoot, path)
		if rErr != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return nil
		}
		if d.IsDir() {
			if isSkippedShadowDir(d.Name()) {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dstShadow, rel), 0750)
		}
		return copyRegularShadowFile(path, filepath.Join(dstShadow, rel), d)
	})
}

func copyRegularShadowFile(srcPath, dstPath string, d fs.DirEntry) error {
	if d.Type()&os.ModeSymlink != 0 {
		return nil
	}
	data, readErr := os.ReadFile(srcPath)
	if readErr != nil {
		return nil
	}
	return os.WriteFile(dstPath, data, 0600)
}

func untarBytesToDir(archive []byte, dstDir string) error {
	absDst, err := filepath.Abs(dstDir)
	if err != nil {
		return err
	}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, rErr := tr.Next()
		if rErr == io.EOF {
			return nil
		}
		if rErr != nil {
			return rErr
		}
		if wErr := extractTarEntry(tr, hdr, absDst); wErr != nil {
			return wErr
		}
	}
}

func extractTarEntry(r io.Reader, hdr *tar.Header, absDst string) error {
	if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeDir {
		return nil
	}
	target, err := filepath.Abs(filepath.Join(absDst, filepath.FromSlash(hdr.Name)))
	if err != nil {
		return nil
	}
	rel, relErr := filepath.Rel(absDst, target)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("tar entry escaped destination")
	}
	if hdr.Typeflag == tar.TypeDir {
		return os.MkdirAll(target, 0750)
	}
	if mkErr := os.MkdirAll(filepath.Dir(target), 0750); mkErr != nil {
		return mkErr
	}
	data, rdErr := io.ReadAll(io.LimitReader(r, 4*1024*1024))
	if rdErr != nil {
		return rdErr
	}
	return os.WriteFile(target, data, 0600)
}
