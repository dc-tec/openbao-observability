package docs

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type VerifyOptions struct {
	RepositoryRoot string
	DocsRoot       string
}

type issue struct {
	Path    string
	Line    int
	Message string
}

type issueList []issue

var (
	headingPattern             = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	referenceDefinitionPattern = regexp.MustCompile(`^\[([^\]]+)\]:\s*(\S+)`)
	referenceLinkPattern       = regexp.MustCompile(`!?\[[^\]]+\]\[([^\]]+)\]`)
	inlineLinkPattern          = regexp.MustCompile(`!?\[([^\]]+)\]\(([^)]+)\)`)
	plainSchemePattern         = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	bannedPhrasePattern        = regexp.MustCompile(
		`(?i)\b(simply|just|easily|quickly|obviously|please)\b|\bnote that\b|\bit should be noted\b|\bas mentioned above\b`,
	)
)

func Verify(opts VerifyOptions) error {
	opts = opts.withDefaults()

	entries, err := markdownFiles(opts.DocsRoot)
	if err != nil {
		return err
	}

	issues := make(issueList, 0, 4)
	for _, path := range entries {
		fileIssues, err := verifyFile(opts.RepositoryRoot, opts.DocsRoot, path)
		if err != nil {
			return err
		}
		issues = append(issues, fileIssues...)
	}

	if len(issues) > 0 {
		return issues
	}

	fmt.Printf("documentation verified under %s (%d files)\n", opts.DocsRoot, len(entries))
	return nil
}

func (o VerifyOptions) withDefaults() VerifyOptions {
	if o.RepositoryRoot == "" {
		o.RepositoryRoot = "."
	}
	if o.DocsRoot == "" {
		o.DocsRoot = filepath.Join(o.RepositoryRoot, "docs")
	}
	o.RepositoryRoot = filepath.Clean(o.RepositoryRoot)
	o.DocsRoot = filepath.Clean(o.DocsRoot)
	return o
}

func markdownFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".md" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk docs: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func verifyFile(repoRoot, docsRoot, path string) (issueList, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	rel := relPath(repoRoot, path)
	refs := map[string]int{}
	lines := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if match := referenceDefinitionPattern.FindStringSubmatch(line); match != nil {
			refs[strings.ToLower(match[1])] = len(lines)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	issues := make(issueList, 0, 4)
	issues = append(issues, verifyFilename(repoRoot, path)...)
	issues = append(issues, verifyStructure(rel, path, lines)...)
	issues = append(issues, verifyStyle(rel, lines)...)
	issues = append(issues, verifyLinks(repoRoot, docsRoot, rel, path, lines, refs)...)
	return issues, nil
}

func verifyFilename(repoRoot, path string) issueList {
	rel := relPath(repoRoot, path)
	base := filepath.Base(path)
	if base == "README.md" {
		return nil
	}
	if base != strings.ToLower(base) {
		return issueList{{Path: rel, Message: "Markdown filenames must be lowercase, except README.md"}}
	}
	if strings.Contains(base, "_") || strings.Contains(base, " ") {
		return issueList{{Path: rel, Message: "Markdown filenames must use hyphen-separated names"}}
	}
	return nil
}

func verifyStructure(rel, path string, lines []string) issueList {
	verifier := structureVerifier{
		rel:      rel,
		path:     path,
		lines:    lines,
		headings: map[string]int{},
	}
	return verifier.verify()
}

type structureVerifier struct {
	rel                string
	path               string
	lines              []string
	issues             issueList
	headings           map[string]int
	h1Count            int
	lastHeadingLevel   int
	hasBeforeYouBegin  bool
	hasVerifyResult    bool
	hasSummary         bool
	seenH1             bool
	inFence            bool
	firstNonEmptyFound bool
}

func (v *structureVerifier) verify() issueList {
	v.checkFirstContentLine()
	if !v.firstNonEmptyFound {
		return issueList{{Path: v.rel, Message: "Markdown file is empty"}}
	}
	v.scanLines()
	v.checkRequiredSections()
	return v.issues
}

func (v *structureVerifier) checkFirstContentLine() {
	for i, line := range v.lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		v.firstNonEmptyFound = true
		if !strings.HasPrefix(line, "# ") {
			v.issues = append(
				v.issues,
				issue{Path: v.rel, Line: i + 1, Message: "first non-empty line must be the H1 title"},
			)
		}
		return
	}
}

func (v *structureVerifier) scanLines() {
	for i, line := range v.lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			v.inFence = !v.inFence
			continue
		}
		if v.inFence {
			continue
		}
		match := headingPattern.FindStringSubmatch(line)
		if match == nil {
			v.checkSummaryParagraph(trimmed)
			continue
		}
		v.checkHeading(lineNo, len(match[1]), strings.TrimSpace(match[2]))
	}
}

func (v *structureVerifier) checkSummaryParagraph(trimmed string) {
	if !v.seenH1 || v.hasSummary || trimmed == "" {
		return
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ">") {
		return
	}
	v.hasSummary = true
}

func (v *structureVerifier) checkHeading(lineNo, level int, text string) {
	if level == 1 {
		v.h1Count++
		v.seenH1 = true
	}
	if v.lastHeadingLevel > 0 && level > v.lastHeadingLevel+1 {
		v.issues = append(v.issues, issue{Path: v.rel, Line: lineNo, Message: "heading levels must not skip"})
	}
	v.lastHeadingLevel = level
	v.checkHeadingText(lineNo, text)
	v.checkDuplicateHeading(lineNo, text)
	v.checkProcedureHeading(level, text)
}

func (v *structureVerifier) checkHeadingText(lineNo int, text string) {
	if strings.ContainsAny(text, "`[]*") {
		v.issues = append(
			v.issues,
			issue{Path: v.rel, Line: lineNo, Message: "headings must not contain code, links, or emphasis"},
		)
	}
	if hasHeadingTerminalPunctuation(text) {
		v.issues = append(v.issues, issue{Path: v.rel, Line: lineNo, Message: "headings must not end with punctuation"})
	}
}

func (v *structureVerifier) checkDuplicateHeading(lineNo int, text string) {
	slug := headingSlug(text)
	if previous, ok := v.headings[slug]; ok {
		v.issues = append(
			v.issues,
			issue{
				Path:    v.rel,
				Line:    lineNo,
				Message: fmt.Sprintf("duplicate heading %q also appears on line %d", text, previous),
			},
		)
	}
	v.headings[slug] = lineNo
}

func (v *structureVerifier) checkProcedureHeading(level int, text string) {
	if level == 2 && text == "Before you begin" {
		v.hasBeforeYouBegin = true
	}
	if level == 2 && text == "Verify the result" {
		v.hasVerifyResult = true
	}
}

func (v *structureVerifier) checkRequiredSections() {
	if v.h1Count != 1 {
		v.issues = append(v.issues, issue{Path: v.rel, Message: fmt.Sprintf("expected exactly one H1, found %d", v.h1Count)})
	}
	if !v.hasSummary {
		v.issues = append(v.issues, issue{Path: v.rel, Message: "page must include a summary paragraph after the H1"})
	}
	if requiresProcedureSections(v.path) {
		v.checkProcedureSections()
	}
}

func (v *structureVerifier) checkProcedureSections() {
	if !v.hasBeforeYouBegin {
		v.issues = append(v.issues, issue{Path: v.rel, Message: "how-to and runbook pages must include ## Before you begin"})
	}
	if !v.hasVerifyResult {
		v.issues = append(v.issues, issue{Path: v.rel, Message: "how-to and runbook pages must include ## Verify the result"})
	}
}

func hasHeadingTerminalPunctuation(text string) bool {
	return strings.HasSuffix(text, ".") || strings.HasSuffix(text, ":") ||
		strings.HasSuffix(text, "?") || strings.HasSuffix(text, "!")
}

func requiresProcedureSections(path string) bool {
	slash := filepath.ToSlash(path)
	if strings.Contains(slash, "/docs/runbooks/") {
		return true
	}
	if strings.HasSuffix(slash, "/docs/docker-compose.md") {
		return true
	}
	if strings.HasSuffix(slash, "/docs/audit/declarative-audit.md") {
		return true
	}
	if strings.HasSuffix(slash, "/docs/metrics/secure-metrics-scrape.md") {
		return true
	}
	if strings.HasSuffix(slash, "/docs/metrics/all-node-metrics-scrape.md") {
		return true
	}
	return false
}

func verifyStyle(rel string, lines []string) issueList {
	var issues issueList
	inFence := false
	for i, line := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				info := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				if info == "" {
					issues = append(
						issues,
						issue{Path: rel, Line: lineNo, Message: "fenced code blocks must include a language tag"},
					)
				}
			}
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.Contains(line, "—") {
			issues = append(issues, issue{Path: rel, Line: lineNo, Message: "do not use em dashes"})
		}
		if match := bannedPhrasePattern.FindString(line); match != "" {
			issues = append(
				issues,
				issue{Path: rel, Line: lineNo, Message: fmt.Sprintf("avoid banned style-guide phrase %q", match)},
			)
		}
	}
	if inFence {
		issues = append(issues, issue{Path: rel, Line: len(lines), Message: "fenced code block is not closed"})
	}
	return issues
}

func verifyLinks(repoRoot, docsRoot, rel, path string, lines []string, refs map[string]int) issueList {
	var issues issueList
	for i, line := range lines {
		lineNo := i + 1
		if referenceDefinitionPattern.MatchString(line) {
			target := referenceDefinitionPattern.FindStringSubmatch(line)[2]
			issues = append(issues, validateTarget(repoRoot, docsRoot, rel, path, lineNo, target)...)
			continue
		}
		for _, match := range referenceLinkPattern.FindAllStringSubmatch(line, -1) {
			id := strings.ToLower(match[1])
			if id == "" {
				continue
			}
			if _, ok := refs[id]; !ok {
				issues = append(
					issues,
					issue{
						Path:    rel,
						Line:    lineNo,
						Message: fmt.Sprintf("reference link %q has no definition", match[1]),
					},
				)
			}
		}
		for _, match := range inlineLinkPattern.FindAllStringSubmatch(line, -1) {
			text := strings.TrimSpace(strings.ToLower(match[1]))
			if text == "here" || text == "this" || text == "link" || text == "click here" {
				issues = append(issues, issue{Path: rel, Line: lineNo, Message: "link text must describe the target"})
			}
			issues = append(issues, validateTarget(repoRoot, docsRoot, rel, path, lineNo, match[2])...)
		}
	}
	return issues
}

func validateTarget(repoRoot, docsRoot, rel, sourcePath string, line int, target string) issueList {
	target = strings.TrimSpace(target)
	target = strings.Trim(target, "<>")
	if target == "" || strings.HasPrefix(target, "#") {
		return nil
	}
	if plainSchemePattern.MatchString(target) {
		if strings.Contains(target, "github.com/dc-tec/openbao-observability") {
			return issueList{
				{Path: rel, Line: line, Message: "internal links must use relative paths, not GitHub URLs"},
			}
		}
		return nil
	}
	if strings.HasPrefix(target, "/") {
		return issueList{{Path: rel, Line: line, Message: "internal links must be relative, not absolute"}}
	}

	targetPath := strings.SplitN(target, "#", 2)[0]
	targetPath = strings.SplitN(targetPath, "?", 2)[0]
	if targetPath == "" {
		return nil
	}

	resolved := filepath.Clean(filepath.Join(filepath.Dir(sourcePath), filepath.FromSlash(targetPath)))
	if _, err := os.Stat(resolved); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return issueList{
				{Path: rel, Line: line, Message: fmt.Sprintf("internal link target %q does not exist", target)},
			}
		}
		return issueList{
			{Path: rel, Line: line, Message: fmt.Sprintf("cannot stat internal link target %q: %v", target, err)},
		}
	}

	if !isInside(docsRoot, resolved) && !isInside(repoRoot, resolved) {
		return issueList{
			{
				Path:    rel,
				Line:    line,
				Message: fmt.Sprintf("internal link target %q resolves outside the repository", target),
			},
		}
	}
	return nil
}

func isInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func headingSlug(text string) string {
	text = strings.ToLower(text)
	var builder strings.Builder
	lastDash := false
	for _, r := range text {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func (issues issueList) Error() string {
	var builder strings.Builder
	limit := len(issues)
	if limit > 20 {
		limit = 20
	}
	fmt.Fprintf(&builder, "documentation verification failed with %d issue(s):", len(issues))
	for i := 0; i < limit; i++ {
		item := issues[i]
		if item.Line > 0 {
			fmt.Fprintf(&builder, "\n- %s:%d: %s", item.Path, item.Line, item.Message)
		} else {
			fmt.Fprintf(&builder, "\n- %s: %s", item.Path, item.Message)
		}
	}
	if len(issues) > limit {
		fmt.Fprintf(&builder, "\n- ... and %d more", len(issues)-limit)
	}
	return builder.String()
}
