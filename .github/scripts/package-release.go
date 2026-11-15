package main

import (
	"archive/zip"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	library := flag.String("library", "", "动态库路径")
	archive := flag.String("archive", "", "发布 ZIP 路径")
	checksum := flag.String("checksum", "", "SHA-256 校验文件路径")
	flag.Parse()
	if err := packageRelease(*library, *archive, *checksum); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func packageRelease(library, archive, checksum string) error {
	if library == "" || archive == "" || checksum == "" {
		return fmt.Errorf("必须提供 library、archive 和 checksum 参数")
	}
	if err := writeArchive(archive, []string{library, "LICENSE"}); err != nil {
		return err
	}
	return writeChecksum(archive, checksum)
}

func writeArchive(path string, sources []string) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	writer := zip.NewWriter(file)
	defer func() { err = errors.Join(err, writer.Close()) }()
	for _, source := range sources {
		if err := addArchiveFile(writer, source); err != nil {
			return err
		}
	}
	return nil
}

func addArchiveFile(writer *zip.Writer, path string) (err error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	// 商店安装器要求动态库和许可文件直接位于 ZIP 根目录。
	header.Name = filepath.Base(path)
	header.Method = zip.Deflate
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, file)
	return err
}

func writeChecksum(archive, checksum string) (err error) {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	line := fmt.Sprintf("%x  %s\n", hash.Sum(nil), filepath.Base(archive))
	return os.WriteFile(checksum, []byte(line), 0644)
}
