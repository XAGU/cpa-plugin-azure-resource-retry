package main

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPackageReleaseUsesStoreLayoutAndArchiveChecksum(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("build", 0755); err != nil {
		t.Fatal(err)
	}
	library := []byte("shared library")
	license := []byte("MIT license")
	if err := os.WriteFile("build/azure-resource-retry.so", library, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("LICENSE", license, 0644); err != nil {
		t.Fatal(err)
	}
	archive := "build/azure-resource-retry_0.1.0_linux_amd64.zip"
	checksum := archive + ".sha256"
	if err := packageRelease("build/azure-resource-retry.so", archive, checksum); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	})
	want := map[string]string{"azure-resource-retry.so": string(library), "LICENSE": string(license)}
	if len(reader.File) != len(want) {
		t.Fatalf("ZIP 文件数 = %d，预期 %d", len(reader.File), len(want))
	}
	for _, file := range reader.File {
		expected, exists := want[file.Name]
		if !exists {
			t.Fatalf("ZIP 根目录出现意外路径 %q", file.Name)
		}
		entry, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(entry)
		errClose := entry.Close()
		if err != nil || errClose != nil {
			t.Fatalf("读取 ZIP 文件失败：%v，关闭失败：%v", err, errClose)
		}
		if string(content) != expected {
			t.Fatalf("%s 内容不匹配", file.Name)
		}
		delete(want, file.Name)
	}
	archiveBytes, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	checksumBytes, err := os.ReadFile(checksum)
	if err != nil {
		t.Fatal(err)
	}
	expected := fmt.Sprintf("%x  %s\n", sha256.Sum256(archiveBytes), filepath.Base(archive))
	if string(checksumBytes) != expected {
		t.Fatalf("校验文件 = %q，预期 %q", checksumBytes, expected)
	}
}
