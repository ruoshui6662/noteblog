package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreBuildsDeterministicPublicTree(t *testing.T) {
	root := t.TempDir()
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
	if len(tree) != 1 || tree[0].Title != "guides" {
		t.Fatalf("tree = %#v, want one guides category", tree)
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

func TestStoreRejectsUnsafeDocumentPaths(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, path := range []string{"../secret.md", "/secret.md", "secret.txt", ""} {
		if _, ok, err := store.Document(path); err == nil || ok {
			t.Fatalf("Document(%q) = ok=%v err=%v, want path error", path, ok, err)
		}
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
