package content

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreBuildsDeterministicPublicTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guides/_category.yml", "title: 使用指南\norder: 5\ncollapsed: true\n")
	writeFile(t, root, "guides/advanced.md", `---
title: 高级用法
order: 20
tags: [Go, Docker]
---
# 高级用法

内容 **可见**。

[危险链接](javascript:alert(1))

<script>alert('blocked')</script>
`)
	writeFile(t, root, "guides/intro.markdown", "---\norder: 10\n---\n# 入门\n\n你好。\n")
	writeFile(t, root, "guides/draft.md", "---\ndraft: true\n---\n# 草稿\n")
	writeFile(t, root, "readme.txt", "ignored")

	store := NewStore(root)
	tree, err := store.Tree()
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Title != "使用指南" || !tree[0].Collapsed {
		t.Fatalf("tree = %#v, want category metadata", tree)
	}
	children := tree[0].Children
	if len(children) != 2 || children[0].Title != "入门" || children[1].Title != "高级用法" {
		t.Fatalf("children = %#v, want public documents sorted by front matter order", children)
	}

	doc, ok, err := store.Document("guides/advanced.md")
	if err != nil || !ok {
		t.Fatalf("Document() = %#v, %v, want document", doc, err)
	}
	if doc.Description != "" || len(doc.Tags) != 2 || !strings.Contains(doc.HTML, "<strong>可见</strong>") {
		t.Fatalf("document metadata or HTML not parsed: %#v", doc)
	}
	if strings.Contains(doc.HTML, "<script>") {
		t.Fatal("raw HTML must not be rendered")
	}
	if strings.Contains(strings.ToLower(doc.HTML), "javascript:") {
		t.Fatal("dangerous URL must not be rendered")
	}
	if len(doc.Headings) != 1 || doc.Headings[0].ID != "高级用法" {
		t.Fatalf("headings = %#v", doc.Headings)
	}
	if _, ok, err := store.Document("guides/draft.md"); err != nil || ok {
		t.Fatalf("draft document should be hidden, got ok=%v err=%v", ok, err)
	}
}

func TestDocumentRewritesRelativeMediaReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guide.md", "# 图像\n\n![Logo](media/logo.png)\n\n![External](https://example.com/logo.png)\n\n![Bad](javascript:alert(1))\n")
	doc, ok, err := NewStore(root).Document("guide.md")
	if err != nil || !ok {
		t.Fatalf("Document() = %#v, %v", doc, err)
	}
	if !strings.Contains(doc.HTML, `src="/media/logo.png"`) {
		t.Fatalf("relative media reference was not rewritten: %s", doc.HTML)
	}
	if !strings.Contains(doc.HTML, `src="https://example.com/logo.png"`) {
		t.Fatalf("external media reference was changed: %s", doc.HTML)
	}
	if strings.Contains(strings.ToLower(doc.HTML), "javascript:") {
		t.Fatalf("dangerous media reference was kept: %s", doc.HTML)
	}
}

func TestResolveMediaPathStaysInsideRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "logo.png", "png")
	resolved, err := ResolveMediaPath(root, "logo.png")
	if err != nil || resolved == "" {
		t.Fatalf("ResolveMediaPath() = %q, %v", resolved, err)
	}
	for _, path := range []string{"../logo.png", "/logo.png", "", "secret.txt:stream"} {
		if _, err := ResolveMediaPath(root, path); err == nil {
			t.Fatalf("ResolveMediaPath(%q) succeeded, want rejection", path)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "outside.txt")
	if err := os.Symlink(outside, link); err == nil {
		if _, err := ResolveMediaPath(root, "outside.txt"); err == nil {
			t.Fatal("symlink outside media root was accepted")
		}
	}
}

func TestStoreRejectsUnsafeDocumentPaths(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, path := range []string{"../secret.md", "/secret.md", "secret.txt", ""} {
		if _, ok, err := store.Document(path); err == nil || ok {
			t.Fatalf("Document(%q) = ok=%v err=%v, want path error", path, ok, err)
		}
	}
}

func TestAdminSourceUsesAtomicVersionedWrites(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	first, err := store.CreateSource("guides/edit.md", []byte("# 第一版\n"))
	if err != nil || first.Hash == "" {
		t.Fatalf("CreateSource() = %#v, %v", first, err)
	}
	source, hash, err := store.ReadSource("guides/edit.md")
	if err != nil || source != "# 第一版\n" || hash != first.Hash {
		t.Fatalf("ReadSource() = %q %q %v", source, hash, err)
	}
	if _, err := store.UpdateSource("guides/edit.md", []byte("# 错误版本\n"), "stale"); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale update error = %v", err)
	}
	second, err := store.UpdateSource("guides/edit.md", []byte("# 第二版\n"), hash)
	if err != nil || second.Hash == first.Hash {
		t.Fatalf("UpdateSource() = %#v, %v", second, err)
	}
	if err := store.DeleteSource("guides/edit.md", hash); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale delete error = %v", err)
	}
	if err := store.DeleteSource("guides/edit.md", second.Hash); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ReadSource("guides/edit.md"); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("deleted source error = %v", err)
	}
}

func TestAdminDocumentsIncludesDraftsAndSourcePaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "drafts/example.md", "---\ndraft: true\nslug: public-example.md\n---\n# 草稿\n")
	documents, err := NewStore(root).AdminDocuments()
	if err != nil || len(documents) != 1 {
		t.Fatalf("AdminDocuments() = %#v, %v", documents, err)
	}
	if documents[0].Path != "drafts/example.md" || !documents[0].Draft {
		t.Fatalf("admin document = %#v", documents[0])
	}
}

func TestMalformedDocumentIsExcludedAndReported(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "broken.md", "---\ntitle: missing close\n# body\n")
	store := NewStore(root)
	if _, err := store.Tree(); err != nil {
		t.Fatal(err)
	}
	if issues := store.Issues(); len(issues) != 1 || issues[0].Path != "broken.md" {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestSearchUsesPublicIndexAndWeightsTitle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "guides/title.md", "---\ntitle: Docker 入门\ntags: [容器]\n---\n# 安装\n\n介绍 Docker。\n")
	writeFile(t, root, "guides/body.md", "# 运行环境\n\nDocker 出现在正文中。\n")
	writeFile(t, root, "guides/draft.md", "---\ndraft: true\n---\n# Docker 草稿\n")

	results, err := NewStore(root).Search("docker")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Path != "guides/title.md" {
		t.Fatalf("results = %#v, want title match first and draft excluded", results)
	}
	if results[0].Snippet == "" || !strings.Contains(strings.ToLower(results[0].Snippet), "docker") {
		t.Fatalf("snippet = %q", results[0].Snippet)
	}
	results, err = NewStore(root).Search(" ")
	if err != nil || len(results) != 0 {
		t.Fatalf("blank search = %#v err=%v", results, err)
	}
	writeFile(t, root, "guides/unsafe.md", "# 公共文档\n\n<script>alert('secret')</script>\n")
	results, err = NewStore(root).Search("secret")
	if err != nil || len(results) != 0 {
		t.Fatalf("raw HTML search = %#v err=%v, want no hidden content", results, err)
	}
}

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
}
