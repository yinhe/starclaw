package archive

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Pack creates a .spore package (tar.gz) from a source directory.
// Returns the SHA256 checksum of the created archive.
func Pack(srcDir, outputPath string) (string, error) {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("create output: %w", err)
	}
	defer outFile.Close()

	hasher := sha256.New()
	multiWriter := io.MultiWriter(outFile, hasher)

	gzWriter := gzip.NewWriter(multiWriter)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	srcDir = filepath.Clean(srcDir)

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			header.Linkname = link
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Write file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tarWriter, file)
		return err
	})

	if err != nil {
		return "", fmt.Errorf("walk source: %w", err)
	}

	// Flush writers
	tarWriter.Close()
	gzWriter.Close()

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

// Unpack extracts a .spore package (tar.gz) to a destination directory.
// Returns the SHA256 checksum of the archive for verification.
func Unpack(sporePath, destDir string) (string, error) {
	inFile, err := os.Open(sporePath)
	if err != nil {
		return "", fmt.Errorf("open spore: %w", err)
	}
	defer inFile.Close()

	hasher := sha256.New()
	teeReader := io.TeeReader(inFile, hasher)

	gzReader, err := gzip.NewReader(teeReader)
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read: %w", err)
		}

		// Sanitize path to prevent directory traversal
		target := filepath.Join(destDir, filepath.FromSlash(header.Name))
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return "", fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			os.Remove(target) // remove existing symlink
			if err := os.Symlink(header.Linkname, target); err != nil {
				return "", err
			}
		}
	}

	// Read remaining data to compute full checksum
	io.Copy(io.Discard, teeReader)

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

// Checksum computes the SHA256 checksum of a file.
func Checksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
