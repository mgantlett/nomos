// Package vscope implements pre-tag GoReleaser template validation, statutory semver
// pin parity, and CHANGELOG.md sanitization airlock checks for Cell 3.3.
package vscope

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

var (
	strictEnvSecretTmplRe = regexp.MustCompile(`^\{\{\s*\.Env\.[A-Z][A-Z0-9_]*\s*\}\}$`)
	unguardedDotEnvRe     = regexp.MustCompile(`\.Env\.[A-Za-z0-9_]+`)
	rootCmdVersionDeclRe  = regexp.MustCompile(`(?m)^\s*var\s+RootCmdVersion\s*=\s*"v?([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)"`)
	changelogTopHeadingRe = regexp.MustCompile(`(?m)^##\s+\[v?([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)\]`)
	pkgbuildVerDeclRe     = regexp.MustCompile(`(?m)^pkgver=([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)`)
	templateSchemaPinRe   = regexp.MustCompile(`raw\.githubusercontent\.com/mgantlett/nomos/v([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)/nomos\.schema\.json`)
	changelogRawSecretRe  = regexp.MustCompile(`(?:-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----|gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16})`)
	changelogBareAtTokRe  = regexp.MustCompile(`(?:^|[\s(])(@[A-Za-z0-9_.-]+)`)
)

type goreleaserAirlockManifest struct {
	Version  int `yaml:"version"`
	Builds   []struct {
		ID      string   `yaml:"id"`
		Ldflags []string `yaml:"ldflags"`
	} `yaml:"builds"`
	Archives []struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"archives"`
	Checksum struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"checksum"`
	HomebrewCasks []struct {
		Name       string `yaml:"name"`
		SkipUpload string `yaml:"skip_upload"`
		Repository struct {
			Token string `yaml:"token"`
		} `yaml:"repository"`
		Hooks struct {
			Post struct {
				InstallSteps string `yaml:"install_steps"`
			} `yaml:"post"`
		} `yaml:"hooks"`
	} `yaml:"homebrew_casks"`
	AURs []struct {
		Name          string `yaml:"name"`
		PrivateKey    string `yaml:"private_key"`
		SkipUpload    string `yaml:"skip_upload"`
		URLTemplate   string `yaml:"url_template"`
		CommitMsgTmpl string `yaml:"commit_msg_template"`
	} `yaml:"aurs"`
}

// EvaluatePreTagReleaseAirlock validates .goreleaser.yaml Go templates and secret-field syntax,
// statutory semver pin parity, and CHANGELOG.md workstation/boundary airlock invariants.
// @intent intent.md task_lifecycle_cli_spec.md #ReleaseNotePayload
func EvaluatePreTagReleaseAirlock(ctx *VerifyContext, evalRoot string) (string, string) {
	if strings.TrimSpace(evalRoot) == "" {
		return "", ""
	}
	if msg, rem := validateGoReleaserManifest(evalRoot); msg != "" {
		return msg, rem
	}
	if msg, rem := validateStatutorySemverParity(evalRoot); msg != "" {
		return msg, rem
	}
	return validateChangelogAirlock(ctx, evalRoot)
}

func goreleaserTemplateFuncMap() template.FuncMap {
	noopStr := func(args ...any) string { return "" }
	noopBool := func(args ...any) bool { return false }
	return template.FuncMap{
		"title": noopStr, "tolower": noopStr, "toupper": noopStr,
		"replace": noopStr, "trimprefix": noopStr, "trimsuffix": noopStr,
		"time": noopStr, "incpatch": noopStr, "incminor": noopStr, "incmajor": noopStr,
		"filter": noopStr, "split": func(string, string) []string { return nil },
		"contains": noopBool, "hasprefix": noopBool, "hassuffix": noopBool,
	}
}

func parseGoReleaserTemplateField(fieldLabel, rawTmpl string) string {
	if !strings.Contains(rawTmpl, "{{") && !strings.Contains(rawTmpl, "}}") {
		return ""
	}
	if _, err := template.New(fieldLabel).Funcs(goreleaserTemplateFuncMap()).Parse(rawTmpl); err != nil {
		return fmt.Sprintf(".goreleaser.yaml invalid Go template in %s: %v", fieldLabel, err)
	}
	return ""
}

func validateGoReleaserManifest(root string) (string, string) {
	grPath := filepath.Join(root, ".goreleaser.yaml")
	if !FileExists(grPath) {
		return "", ""
	}
	raw, err := os.ReadFile(grPath)
	if err != nil {
		return fmt.Sprintf("failed reading .goreleaser.yaml: %v", err), "ensure .goreleaser.yaml is readable"
	}
	var m goreleaserAirlockManifest
	if yErr := yaml.Unmarshal(raw, &m); yErr != nil {
		return fmt.Sprintf("malformed .goreleaser.yaml YAML syntax: %v", yErr), "fix YAML syntax in .goreleaser.yaml before tagging a release"
	}
	if issue := checkGoReleaserTemplates(&m); issue != "" {
		return issue, "fix Go template syntax in .goreleaser.yaml before tagging a release"
	}
	if issue := checkGoReleaserSecretFields(&m); issue != "" {
		return issue, "use pure '{{ .Env.<VAR> }}' on secret fields and 'index .Env \"<VAR>\"' on skip_upload guards in .goreleaser.yaml"
	}
	return "", ""
}

func checkGoReleaserTemplates(m *goreleaserAirlockManifest) string {
	type namedTmpl struct{ label, val string }
	var fields []namedTmpl
	for i, b := range m.Builds {
		for j, lf := range b.Ldflags {
			fields = append(fields, namedTmpl{fmt.Sprintf("builds[%d].ldflags[%d]", i, j), lf})
		}
	}
	for i, a := range m.Archives {
		fields = append(fields, namedTmpl{fmt.Sprintf("archives[%d].name_template", i), a.NameTemplate})
	}
	fields = append(fields, namedTmpl{"checksum.name_template", m.Checksum.NameTemplate})
	for i, c := range m.HomebrewCasks {
		fields = append(fields,
			namedTmpl{fmt.Sprintf("homebrew_casks[%d].repository.token", i), c.Repository.Token},
			namedTmpl{fmt.Sprintf("homebrew_casks[%d].skip_upload", i), c.SkipUpload},
			namedTmpl{fmt.Sprintf("homebrew_casks[%d].hooks.post.install_steps", i), c.Hooks.Post.InstallSteps},
		)
	}
	for i, a := range m.AURs {
		fields = append(fields,
			namedTmpl{fmt.Sprintf("aurs[%d].private_key", i), a.PrivateKey},
			namedTmpl{fmt.Sprintf("aurs[%d].skip_upload", i), a.SkipUpload},
			namedTmpl{fmt.Sprintf("aurs[%d].url_template", i), a.URLTemplate},
			namedTmpl{fmt.Sprintf("aurs[%d].commit_msg_template", i), a.CommitMsgTmpl},
		)
	}
	for _, f := range fields {
		if issue := parseGoReleaserTemplateField(f.label, f.val); issue != "" {
			return issue
		}
	}
	return ""
}

func checkGoReleaserSecretFields(m *goreleaserAirlockManifest) string {
	for _, c := range m.HomebrewCasks {
		tok := strings.TrimSpace(c.Repository.Token)
		if tok != "" && !strictEnvSecretTmplRe.MatchString(tok) {
			return fmt.Sprintf(".goreleaser.yaml homebrew_casks[%s].repository.token %q violates GoReleaser v2.20+ secret-only template syntax", c.Name, tok)
		}
		if unguardedDotEnvRe.MatchString(c.SkipUpload) {
			return fmt.Sprintf(".goreleaser.yaml homebrew_casks[%s].skip_upload uses unguarded .Env.<KEY> lookup", c.Name)
		}
	}
	for _, a := range m.AURs {
		pk := strings.TrimSpace(a.PrivateKey)
		if pk != "" && !strictEnvSecretTmplRe.MatchString(pk) {
			return fmt.Sprintf(".goreleaser.yaml aurs[%s].private_key %q violates GoReleaser v2.20+ secret-only template syntax", a.Name, pk)
		}
		if unguardedDotEnvRe.MatchString(a.SkipUpload) {
			return fmt.Sprintf(".goreleaser.yaml aurs[%s].skip_upload uses unguarded .Env.<KEY> lookup", a.Name)
		}
	}
	return ""
}

func validateStatutorySemverParity(root string) (string, string) {
	rootGoPath := filepath.Join(root, "substrate", "cli", "root.go")
	if !FileExists(rootGoPath) {
		return "", ""
	}
	rootBytes, err := os.ReadFile(rootGoPath)
	if err != nil {
		return "", ""
	}
	m := rootCmdVersionDeclRe.FindSubmatch(rootBytes)
	if len(m) < 2 {
		return "", ""
	}
	wantVer := string(m[1])
	remedy := fmt.Sprintf("align RootCmdVersion (%s), CHANGELOG.md top heading, templates/nomos.yaml schema tag, and packaging/aur/PKGBUILD pkgver before tagging", wantVer)

	if issue := checkChangelogSemverPin(root, wantVer); issue != "" {
		return issue, remedy
	}
	if issue := checkTemplateSchemaSemverPin(root, wantVer); issue != "" {
		return issue, remedy
	}
	if issue := checkPKGBUILDVersionPin(root, wantVer); issue != "" {
		return issue, remedy
	}
	return "", ""
}

func checkChangelogSemverPin(root, wantVer string) string {
	clPath := filepath.Join(root, "CHANGELOG.md")
	if !FileExists(clPath) {
		return ""
	}
	data, err := os.ReadFile(clPath)
	if err != nil {
		return ""
	}
	if cm := changelogTopHeadingRe.FindSubmatch(data); len(cm) >= 2 && string(cm[1]) != wantVer {
		return fmt.Sprintf("statutory semver pin drift: root.go RootCmdVersion=%s != CHANGELOG.md top release v%s", wantVer, string(cm[1]))
	}
	return ""
}

func checkTemplateSchemaSemverPin(root, wantVer string) string {
	tmplPath := filepath.Join(root, "substrate", "cli", "templates", "nomos.yaml")
	if !FileExists(tmplPath) {
		return ""
	}
	data, err := os.ReadFile(tmplPath)
	if err != nil {
		return ""
	}
	if tm := templateSchemaPinRe.FindSubmatch(data); len(tm) >= 2 && string(tm[1]) != wantVer {
		return fmt.Sprintf("statutory semver pin drift: root.go RootCmdVersion=%s != templates/nomos.yaml schema pin v%s", wantVer, string(tm[1]))
	}
	return ""
}

func checkPKGBUILDVersionPin(root, wantVer string) string {
	pkgPath := filepath.Join(root, "packaging", "aur", "PKGBUILD")
	if !FileExists(pkgPath) {
		return ""
	}
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return ""
	}
	if pm := pkgbuildVerDeclRe.FindSubmatch(data); len(pm) >= 2 && string(pm[1]) != wantVer {
		return fmt.Sprintf("statutory semver pin drift: root.go RootCmdVersion=%s != packaging/aur/PKGBUILD pkgver=%s", wantVer, string(pm[1]))
	}
	return ""
}

func validateChangelogAirlock(ctx *VerifyContext, root string) (string, string) {
	clPath := filepath.Join(root, "CHANGELOG.md")
	if !FileExists(clPath) {
		return "", ""
	}
	rawBytes, err := os.ReadFile(clPath)
	if err != nil {
		return "", ""
	}
	text := string(rawBytes)
	remedy := "scrub workstation paths, credentials, prohibited boundary packages, and Unicode em/en-dashes from CHANGELOG.md"

	if changelogRawSecretRe.MatchString(text) {
		return "CHANGELOG.md airlock rejected raw credential or private key token", remedy
	}
	if tok := detectChangelogHostPathLeak(text); tok != "" {
		return fmt.Sprintf("CHANGELOG.md airlock rejected workstation or local URI path prefix %q (Law 5)", tok), remedy
	}
	if msg := detectChangelogBoundaryViolation(ctx, text); msg != "" {
		return msg, remedy
	}
	if msg := detectStrictChangelogTypographyDrift(ctx, text); msg != "" {
		return msg, remedy
	}
	return "", ""
}

func detectChangelogBoundaryViolation(ctx *VerifyContext, text string) string {
	cfg := ResolveVerifyConfig(ctx)
	if cfg == nil || !cfg.Governance.Boundary.Enforce {
		return ""
	}
	for _, pkg := range cfg.Governance.Boundary.ProhibitedPackages {
		trimmed := strings.TrimSpace(pkg)
		if trimmed != "" && strings.Contains(text, trimmed) {
			return fmt.Sprintf("CHANGELOG.md airlock rejected prohibited boundary package %q (Law 7)", trimmed)
		}
	}
	return ""
}

func detectStrictChangelogTypographyDrift(ctx *VerifyContext, text string) string {
	if IsBrownfieldRatchetActive(ctx) {
		return ""
	}
	if strings.ContainsRune(text, '\u2014') || strings.ContainsRune(text, '\u2013') {
		return "CHANGELOG.md airlock rejected Unicode em-dash or en-dash formatting drift; use spaced ASCII ' - '"
	}
	if atTok := detectBareChangelogAtMention(text); atTok != "" {
		return fmt.Sprintf("CHANGELOG.md airlock rejected unbackticked GitHub @mention token %q; wrap annotation in backticks", atTok)
	}
	return ""
}

func detectBareChangelogAtMention(text string) string {
	for _, line := range strings.Split(text, "\n") {
		segs := strings.Split(line, "`")
		for i := 0; i < len(segs); i += 2 {
			if m := changelogBareAtTokRe.FindStringSubmatch(segs[i]); len(m) == 2 {
				return m[1]
			}
		}
	}
	return ""
}

func detectChangelogHostPathLeak(text string) string {
	sep := "/"
	prefixes := []string{
		sep + "home" + sep,
		sep + "Users" + sep,
		sep + "var" + sep + "home" + sep,
		sep + "workstations" + sep,
		"file:" + sep + sep + sep,
		`C:\Users\`,
	}
	for _, p := range prefixes {
		if strings.Contains(text, p) {
			return p
		}
	}
	return ""
}

