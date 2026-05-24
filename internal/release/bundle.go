package release

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const projectName = "openbao-observability"

type BundleOptions struct {
	RepositoryRoot  string
	Version         string
	OutputPath      string
	SourceDateEpoch int64
	Includes        []string
}

type ChecksumOptions struct {
	Directory  string
	OutputPath string
}

type bundleEntry struct {
	AbsolutePath string
	RelativePath string
	Info         fs.FileInfo
}

var (
	versionPattern  = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	defaultIncludes = []string{
		"README.md",
		"LICENSE",
		"SECURITY.md",
		"CHANGELOG.md",
		"CONTRIBUTING.md",
		"contracts",
		"dashboards",
		"docs",
		"examples",
		"generated",
	}
)

func Bundle(opts BundleOptions) error {
	opts, err := opts.withDefaults()
	if err != nil {
		return err
	}

	entries, err := collectBundleEntries(opts.RepositoryRoot, opts.Includes)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("release bundle has no files")
	}

	return writeBundle(opts, entries)
}

func Checksums(opts ChecksumOptions) error {
	opts, err := opts.withDefaults()
	if err != nil {
		return err
	}

	entries, err := checksumEntries(opts.Directory, opts.OutputPath)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("no release assets found under %s", opts.Directory)
	}

	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return fmt.Errorf("create checksum directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(opts.OutputPath), "."+filepath.Base(opts.OutputPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create checksum temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	for _, entry := range entries {
		sum, err := fileSHA256(entry.AbsolutePath)
		if err != nil {
			_ = tmp.Close()
			return err
		}
		if _, err := fmt.Fprintf(tmp, "%s  %s\n", sum, filepath.ToSlash(entry.RelativePath)); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("write checksum: %w", err)
		}
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close checksum temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), opts.OutputPath); err != nil {
		return fmt.Errorf("move checksum file into place: %w", err)
	}
	return nil
}

func (o BundleOptions) withDefaults() (BundleOptions, error) {
	if o.RepositoryRoot == "" {
		o.RepositoryRoot = "."
	}
	root, err := filepath.Abs(o.RepositoryRoot)
	if err != nil {
		return o, fmt.Errorf("resolve repository root: %w", err)
	}
	o.RepositoryRoot = filepath.Clean(root)

	if o.Version == "" {
		o.Version = "0.0.0-dev"
	}
	if !versionPattern.MatchString(o.Version) {
		return o, fmt.Errorf("release version %q must be SemVer without a leading v", o.Version)
	}

	if o.OutputPath == "" {
		o.OutputPath = filepath.Join("dist", "release", projectName+"_"+o.Version+".tar.gz")
	}
	if !filepath.IsAbs(o.OutputPath) {
		o.OutputPath = filepath.Join(o.RepositoryRoot, o.OutputPath)
	}
	o.OutputPath = filepath.Clean(o.OutputPath)

	if len(o.Includes) == 0 {
		o.Includes = append([]string(nil), defaultIncludes...)
	}
	for i, include := range o.Includes {
		cleaned := filepath.Clean(include)
		if cleaned == "." || filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
			return o, fmt.Errorf("release include path %q must be relative to the repository root", include)
		}
		o.Includes[i] = cleaned
	}
	return o, nil
}

func (o ChecksumOptions) withDefaults() (ChecksumOptions, error) {
	if o.Directory == "" {
		o.Directory = filepath.Join("dist", "release")
	}
	dir, err := filepath.Abs(o.Directory)
	if err != nil {
		return o, fmt.Errorf("resolve release directory: %w", err)
	}
	o.Directory = filepath.Clean(dir)

	if o.OutputPath == "" {
		o.OutputPath = filepath.Join(o.Directory, "checksums.txt")
	}
	if !filepath.IsAbs(o.OutputPath) {
		output, err := filepath.Abs(o.OutputPath)
		if err != nil {
			return o, fmt.Errorf("resolve checksum output path: %w", err)
		}
		o.OutputPath = output
	}
	o.OutputPath = filepath.Clean(o.OutputPath)
	return o, nil
}

func collectBundleEntries(root string, includes []string) ([]bundleEntry, error) {
	seen := map[string]bool{}
	var entries []bundleEntry

	for _, include := range includes {
		includePath := filepath.Join(root, include)
		info, err := os.Lstat(includePath)
		if err != nil {
			return nil, fmt.Errorf("inspect release include %s: %w", include, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("release include %s is a symlink; symlinks are not supported in release bundles", include)
		}
		if !info.IsDir() {
			entry, err := bundleEntryFor(root, includePath, info)
			if err != nil {
				return nil, err
			}
			if !seen[entry.RelativePath] {
				seen[entry.RelativePath] = true
				entries = append(entries, entry)
			}
			continue
		}

		err = filepath.WalkDir(includePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}
				return fmt.Errorf("release include %s is a symlink; symlinks are not supported in release bundles", filepath.ToSlash(rel))
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			entry, err := bundleEntryFor(root, path, info)
			if err != nil {
				return err
			}
			if !seen[entry.RelativePath] {
				seen[entry.RelativePath] = true
				entries = append(entries, entry)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk release include %s: %w", include, err)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].RelativePath < entries[j].RelativePath
	})
	return entries, nil
}

func bundleEntryFor(root, absolutePath string, info fs.FileInfo) (bundleEntry, error) {
	rel, err := filepath.Rel(root, absolutePath)
	if err != nil {
		return bundleEntry{}, fmt.Errorf("resolve bundle relative path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return bundleEntry{}, fmt.Errorf("bundle path %s escapes repository root", absolutePath)
	}
	return bundleEntry{
		AbsolutePath: absolutePath,
		RelativePath: rel,
		Info:         info,
	}, nil
}

func writeBundle(opts BundleOptions, entries []bundleEntry) error {
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return fmt.Errorf("create release output directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(opts.OutputPath), "."+filepath.Base(opts.OutputPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create release bundle temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	epoch := time.Unix(opts.SourceDateEpoch, 0).UTC()
	gzipWriter := gzip.NewWriter(tmp)
	gzipWriter.Name = ""
	gzipWriter.Comment = ""
	gzipWriter.ModTime = epoch
	tarWriter := tar.NewWriter(gzipWriter)
	rootName := projectName + "_" + opts.Version

	if err := tarWriter.WriteHeader(&tar.Header{
		Name:       rootName + "/",
		Typeflag:   tar.TypeDir,
		Mode:       0o755,
		ModTime:    epoch,
		AccessTime: epoch,
		ChangeTime: epoch,
		Uid:        0,
		Gid:        0,
	}); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write release bundle root header: %w", err)
	}

	for _, entry := range entries {
		if err := writeBundleEntry(tarWriter, rootName, entry, epoch); err != nil {
			_ = tmp.Close()
			return err
		}
	}

	if err := tarWriter.Close(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("close gzip writer: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close release bundle temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), opts.OutputPath); err != nil {
		return fmt.Errorf("move release bundle into place: %w", err)
	}
	return nil
}

func writeBundleEntry(tarWriter *tar.Writer, rootName string, entry bundleEntry, epoch time.Time) error {
	file, err := os.Open(entry.AbsolutePath)
	if err != nil {
		return fmt.Errorf("open release bundle file %s: %w", entry.RelativePath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	mode := int64(entry.Info.Mode().Perm())
	if mode == 0 {
		mode = 0o644
	}
	header := &tar.Header{
		Name:       path.Join(rootName, filepath.ToSlash(entry.RelativePath)),
		Typeflag:   tar.TypeReg,
		Mode:       mode,
		Size:       entry.Info.Size(),
		ModTime:    epoch,
		AccessTime: epoch,
		ChangeTime: epoch,
		Uid:        0,
		Gid:        0,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("write release bundle header for %s: %w", entry.RelativePath, err)
	}
	if _, err := io.Copy(tarWriter, file); err != nil {
		return fmt.Errorf("write release bundle file %s: %w", entry.RelativePath, err)
	}
	return nil
}

func checksumEntries(dir, outputPath string) ([]bundleEntry, error) {
	var entries []bundleEntry
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if filepath.Clean(abs) == outputPath {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		entry, err := bundleEntryFor(dir, path, info)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk release assets: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].RelativePath < entries[j].RelativePath
	})
	return entries, nil
}

func fileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open release asset %s: %w", filePath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash release asset %s: %w", filePath, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
