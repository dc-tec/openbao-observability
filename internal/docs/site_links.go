package docs

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type SiteLinkOptions struct {
	SiteRoot string
	BaseURL  string
	BasePath string
}

var htmlURLAttributePattern = regexp.MustCompile(`(?i)\b(?:href|src)=("[^"]*"|'[^']*'|[^\s>]+)`)

func VerifySiteLinks(opts SiteLinkOptions) error {
	opts, err := opts.withDefaults()
	if err != nil {
		return err
	}

	var htmlFiles []string
	err = filepath.WalkDir(opts.SiteRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".html" {
			htmlFiles = append(htmlFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk generated site: %w", err)
	}

	var issues issueList
	for _, path := range htmlFiles {
		fileIssues, err := verifySiteFileLinks(opts, path)
		if err != nil {
			return err
		}
		issues = append(issues, fileIssues...)
	}
	if len(issues) > 0 {
		return issues
	}

	fmt.Printf("generated site links verified under %s (%d files)\n", opts.SiteRoot, len(htmlFiles))
	return nil
}

func (o SiteLinkOptions) withDefaults() (SiteLinkOptions, error) {
	if o.SiteRoot == "" {
		o.SiteRoot = "public"
	}
	o.SiteRoot = filepath.Clean(o.SiteRoot)

	if o.BasePath == "" && o.BaseURL != "" {
		parsed, err := url.Parse(o.BaseURL)
		if err != nil {
			return o, fmt.Errorf("parse base URL: %w", err)
		}
		o.BasePath = parsed.EscapedPath()
	}
	if o.BasePath == "" {
		o.BasePath = "/"
	}
	if !strings.HasPrefix(o.BasePath, "/") {
		o.BasePath = "/" + o.BasePath
	}
	if !strings.HasSuffix(o.BasePath, "/") {
		o.BasePath += "/"
	}
	return o, nil
}

func verifySiteFileLinks(opts SiteLinkOptions, path string) (issueList, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read generated site file %s: %w", path, err)
	}

	rel := relPath(opts.SiteRoot, path)
	var issues issueList
	for _, match := range htmlURLAttributePattern.FindAllSubmatch(data, -1) {
		target := strings.Trim(string(match[1]), `"'`)
		issues = append(issues, validateGeneratedSiteTarget(opts, rel, target)...)
	}
	return issues, nil
}

func validateGeneratedSiteTarget(opts SiteLinkOptions, rel, target string) issueList {
	if target == "" || strings.HasPrefix(target, "#") || isExternalGeneratedTarget(target) {
		return nil
	}
	if !strings.HasPrefix(target, "/") {
		return nil
	}
	if opts.BasePath != "/" && !strings.HasPrefix(target, opts.BasePath) {
		return issueList{{Path: rel, Message: fmt.Sprintf("absolute site link %q does not include base path %q", target, opts.BasePath)}}
	}

	targetPath := strings.SplitN(target, "#", 2)[0]
	targetPath = strings.SplitN(targetPath, "?", 2)[0]
	if targetPath == "" {
		return nil
	}

	sitePath := strings.TrimPrefix(targetPath, opts.BasePath)
	if opts.BasePath == "/" {
		sitePath = strings.TrimPrefix(targetPath, "/")
	}
	if sitePath == "" {
		sitePath = "index.html"
	} else if strings.HasSuffix(sitePath, "/") {
		sitePath = filepath.Join(filepath.FromSlash(sitePath), "index.html")
	} else if filepath.Ext(sitePath) == "" {
		sitePath = filepath.Join(filepath.FromSlash(sitePath), "index.html")
	} else {
		sitePath = filepath.FromSlash(sitePath)
	}

	resolved := filepath.Clean(filepath.Join(opts.SiteRoot, sitePath))
	if !isInside(opts.SiteRoot, resolved) {
		return issueList{{Path: rel, Message: fmt.Sprintf("absolute site link %q resolves outside generated site root", target)}}
	}
	if _, err := os.Stat(resolved); err != nil {
		return issueList{{Path: rel, Message: fmt.Sprintf("absolute site link %q target does not exist", target)}}
	}
	return nil
}

func isExternalGeneratedTarget(target string) bool {
	lower := strings.ToLower(target)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(target, "//")
}
