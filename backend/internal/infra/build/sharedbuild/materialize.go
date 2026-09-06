package sharedbuild

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/registry"
)

// maxSharedUnpackBytes bounds a single shared tarball so a compromised registry
// cannot blow the generator's disk with one entry (mirrors the site layout's
// safety cap).
const maxSharedUnpackBytes = 512 << 20 // 512 MiB

// materialize fetches every locked package of the frozen lock, verifies its
// sha512 integrity via the registry client and unpacks it under dir/node_modules
// — a DB-free mirror of layout.MaterializeDeps that the standalone generator can
// run without the application's dep_packages cache. byKey carries the registry
// metadata (tarball URL) that the lock itself does not.
func materialize(ctx context.Context, client *registry.Client, dir string, result resolveResult) error {
	names := make([]string, 0, len(result.lock.Deps))
	for _, dep := range result.lock.Deps {
		names = append(names, dep.Name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, dep := range result.lock.Deps {
			if dep.Name != name {
				continue
			}
			resolved, ok := result.byKey[dep.InstanceKey()]
			if !ok {
				return fmt.Errorf("shared materialize %s@%s: no registry metadata", dep.Name, dep.Version)
			}
			if err := fetchAndUnpack(ctx, client, dir, dep, resolved); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func fetchAndUnpack(ctx context.Context, client *registry.Client, dir string, dep domain.LockedDep, resolved registry.ResolvedVersion) error {
	data, err := client.Fetch(ctx, resolved)
	if err != nil {
		return fmt.Errorf("shared fetch %s@%s: %w", dep.Name, dep.Version, err)
	}
	target := filepath.Join(dir, "node_modules", filepath.FromSlash(dep.Name))
	if err := unpackTarball(data, target); err != nil {
		return fmt.Errorf("shared unpack %s@%s: %w", dep.Name, dep.Version, err)
	}
	return nil
}

// unpackTarball extracts an npm tarball into target, stripping the "package/"
// prefix npm tarballs carry and rejecting any entry that would escape target
// (path traversal, absolute paths). Transitive deps nested inside a tarball's
// node_modules/ are skipped (flat layout, spec §5).
func unpackTarball(data []byte, target string) error {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer reader.Close()

	var total int64
	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Size < 0 {
			return fmt.Errorf("negative size for %s", hdr.Name)
		}
		total += hdr.Size
		if total > maxSharedUnpackBytes {
			return fmt.Errorf("exceeds %d MiB safety cap", maxSharedUnpackBytes>>20)
		}

		rel := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "package/")
		if rel == "" || strings.HasPrefix(rel, "node_modules/") {
			continue
		}
		dst := filepath.Join(target, filepath.FromSlash(rel))
		if !within(target, dst) {
			return fmt.Errorf("entry %q escapes the package directory", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.CopyN(file, tr, hdr.Size); err != nil {
				file.Close()
				return fmt.Errorf("%s: %w", hdr.Name, err)
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// within reports whether child is inside parent (lexically, after cleaning).
func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}
