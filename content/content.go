// Package content owns the file-backed document index.
//
// Markdown files are the source of truth. The index is disposable and can be
// rebuilt from the content directory at any time.
package content

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

type Heading struct {
	Level int    `json:"level"`
	ID    string `json:"id"`
	Text  string `json:"text"`
}

type Document struct {
	Path        string    `json:"path"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Hash        string    `json:"hash"`
	HTML        string    `json:"html"`
	Headings    []Heading `json:"headings,omitempty"`
	Draft       bool      `json:"-"`
	order       int
	searchText  string
	sourcePath  string
}

type Node struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Collapsed   bool   `json:"collapsed,omitempty"`
	Children    []Node `json:"children,omitempty"`
}

// Category is the editable filesystem-backed metadata for a document folder.
// The directory itself is the category identity; _category.yml only controls
// its presentation in the tree and on the discovery page.
type Category struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Order       int    `json:"order"`
	Collapsed   bool   `json:"collapsed"`
}

type Issue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type SearchResult struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Snippet     string `json:"snippet"`
	score       int
}

type Store struct {
	root    string
	mu      sync.RWMutex
	writeMu sync.Mutex
	docs    map[string]Document
	issues  []Issue
}

func NewStore(root string) *Store {
	return &Store{root: root, docs: make(map[string]Document)}
}

// Refresh rebuilds the disposable index without modifying any source file.
// Invalid individual documents are excluded and reported as issues so one bad
// file cannot take the public reader offline.
func (s *Store) Refresh() error {
	docs, issues, err := scan(s.root)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.docs = docs
	s.issues = issues
	s.mu.Unlock()
	return nil
}

func (s *Store) Issues() []Issue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Issue(nil), s.issues...)
}

func (s *Store) Tree() ([]Node, error) {
	if err := s.Refresh(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	root := &treeNode{node: Node{Kind: "category", Path: "", Title: "文档库"}}
	for _, doc := range s.docs {
		parts := strings.Split(doc.Path, "/")
		current := root
		for index, part := range parts[:len(parts)-1] {
			categoryPath := strings.Join(parts[:index+1], "/")
			child := current.find(categoryPath)
			if child == nil {
				metadata := readCategoryMetadata(s.root, categoryPath)
				child = &treeNode{node: Node{
					Kind:        "category",
					Path:        categoryPath,
					Title:       firstNonEmpty(metadata.Title, titleFromFilename(part)),
					Description: metadata.Description,
					Collapsed:   metadata.Collapsed,
				}, order: metadata.Order}
				current.children = append(current.children, child)
			}
			current = child
		}
		current.children = append(current.children, &treeNode{node: Node{
			Kind:  "document",
			Path:  doc.Path,
			Title: doc.Title,
		}, order: doc.order})
	}
	sortTree(root)
	result := make([]Node, 0, len(root.children))
	for _, child := range root.children {
		result = append(result, child.node)
	}
	return result, nil
}

func (s *Store) Document(relativePath string) (Document, bool, error) {
	clean, err := cleanDocumentPath(relativePath)
	if err != nil {
		return Document{}, false, err
	}
	if err := s.Refresh(); err != nil {
		return Document{}, false, err
	}
	s.mu.RLock()
	doc, ok := s.docs[clean]
	s.mu.RUnlock()
	return doc, ok, nil
}

func (s *Store) Search(query string) ([]SearchResult, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []SearchResult{}, nil
	}
	if err := s.Refresh(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	results := make([]SearchResult, 0)
	for _, doc := range s.docs {
		score := 0
		if strings.Contains(strings.ToLower(doc.Title), query) {
			score += 8
		}
		for _, tag := range doc.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				score += 6
			}
		}
		for _, heading := range doc.Headings {
			if strings.Contains(strings.ToLower(heading.Text), query) {
				score += 4
			}
		}
		if strings.Contains(strings.ToLower(doc.Description), query) {
			score += 3
		}
		if strings.Contains(strings.ToLower(doc.searchText), query) {
			score++
		}
		if score == 0 {
			continue
		}
		results = append(results, SearchResult{
			Path:        doc.Path,
			Title:       doc.Title,
			Description: doc.Description,
			Snippet:     snippet(doc.searchText, query),
			score:       score,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return strings.ToLower(results[i].Title) < strings.ToLower(results[j].Title)
	})
	if len(results) > 20 {
		results = results[:20]
	}
	return results, nil
}

// AdminDocuments returns every valid source document, including drafts, for
// the authenticated management surface. The public index intentionally keeps
// drafts out of Tree, Document, and Search.
func (s *Store) AdminDocuments() ([]Document, error) {
	docs, _, err := scanWithDrafts(s.root, true)
	if err != nil {
		return nil, err
	}
	result := make([]Document, 0, len(docs))
	for _, doc := range docs {
		doc.Path = doc.sourcePath
		result = append(result, doc)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}

// AdminCategories returns every non-hidden directory beneath the content
// root. A directory is a category even when it currently has no Markdown
// documents, which lets the admin UI prepare a taxonomy before writing notes.
func (s *Store) AdminCategories() ([]Category, error) {
	result := make([]Category, 0)
	err := filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path == s.root {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}
		relative, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		categoryPath := filepath.ToSlash(relative)
		metadata := readCategoryMetadata(s.root, categoryPath)
		result = append(result, Category{
			Path:        categoryPath,
			Title:       firstNonEmpty(metadata.Title, titleFromFilename(entry.Name())),
			Description: metadata.Description,
			Order:       metadata.Order,
			Collapsed:   metadata.Collapsed,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Order != result[j].Order {
			return result[i].Order < result[j].Order
		}
		return result[i].Path < result[j].Path
	})
	return result, nil
}

// UpsertCategory writes the presentation metadata for a category and creates
// its directory when it does not exist yet. Markdown documents remain the
// source of truth; this file only describes how the directory is presented.
func (s *Store) UpsertCategory(category Category) error {
	path, err := cleanCategoryPath(category.Path)
	if err != nil {
		return err
	}
	category.Title = strings.TrimSpace(category.Title)
	category.Description = strings.TrimSpace(category.Description)
	if category.Title == "" {
		return ErrInvalidCategory
	}
	if len(category.Title) > 160 || len(category.Description) > 500 {
		return ErrInvalidCategory
	}
	categoryPath, err := resolveCategoryPath(s.root, path)
	if err != nil {
		return err
	}
	metadata, err := yaml.Marshal(categoryMetadata{
		Title:       category.Title,
		Description: category.Description,
		Order:       category.Order,
		Collapsed:   category.Collapsed,
	})
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := os.MkdirAll(categoryPath, 0o750); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(categoryPath, "_category.yml"), metadata); err != nil {
		return err
	}
	return s.Refresh()
}

// ReadSource reads the Markdown source and returns its content hash. The hash
// is the optimistic-lock version used by the admin API's If-Match header.
func (s *Store) ReadSource(relativePath string) (string, string, error) {
	path, err := cleanDocumentPath(relativePath)
	if err != nil {
		return "", "", err
	}
	fullPath, err := resolveDocumentPath(s.root, path)
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", ErrDocumentNotFound
		}
		return "", "", err
	}
	return string(data), hashBytes(data), nil
}

var (
	ErrDocumentExists     = errors.New("document already exists")
	ErrDocumentNotFound   = errors.New("document not found")
	ErrVersionConflict    = errors.New("document version conflict")
	ErrInvalidDocument    = errors.New("document content is invalid")
	ErrUnsafeDocumentPath = errors.New("document path is unsafe")
	ErrInvalidCategory    = errors.New("category metadata is invalid")
	ErrUnsafeCategoryPath = errors.New("category path is unsafe")
)

// CreateSource validates and atomically creates a Markdown document.
func (s *Store) CreateSource(relativePath string, data []byte) (Document, error) {
	return s.writeSource(relativePath, data, "", true)
}

// UpdateSource validates and atomically updates a Markdown document when its
// current hash matches expectedHash.
func (s *Store) UpdateSource(relativePath string, data []byte, expectedHash string) (Document, error) {
	if strings.TrimSpace(expectedHash) == "" {
		return Document{}, ErrVersionConflict
	}
	return s.writeSource(relativePath, data, expectedHash, false)
}

// DeleteSource removes a Markdown document when its current hash matches
// expectedHash.
func (s *Store) DeleteSource(relativePath, expectedHash string) error {
	path, err := cleanDocumentPath(relativePath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(expectedHash) == "" {
		return ErrVersionConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	fullPath, err := resolveDocumentPath(s.root, path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrDocumentNotFound
		}
		return err
	}
	if expectedHash != hashBytes(data) {
		return ErrVersionConflict
	}
	if err := os.Remove(fullPath); err != nil {
		return err
	}
	return s.Refresh()
}

func (s *Store) writeSource(relativePath string, data []byte, expectedHash string, createOnly bool) (Document, error) {
	path, err := cleanDocumentPath(relativePath)
	if err != nil {
		return Document{}, err
	}
	doc, err := parseDocument(path, data)
	if err != nil {
		return Document{}, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	doc.Path = path
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	fullPath, err := resolveDocumentPath(s.root, path)
	if err != nil {
		return Document{}, err
	}
	current, readErr := os.ReadFile(fullPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return Document{}, readErr
	}
	exists := readErr == nil
	if createOnly && exists {
		return Document{}, ErrDocumentExists
	}
	if !createOnly && !exists {
		return Document{}, ErrDocumentNotFound
	}
	if !createOnly && expectedHash != hashBytes(current) {
		return Document{}, ErrVersionConflict
	}
	if err := atomicWrite(fullPath, data); err != nil {
		return Document{}, err
	}
	if err := s.Refresh(); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func atomicWrite(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".noteblog-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o640); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func resolveDocumentPath(root, relativePath string) (string, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(absoluteRoot, filepath.FromSlash(relativePath))
	relative, err := filepath.Rel(absoluteRoot, fullPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrUnsafeDocumentPath
	}
	parts := strings.Split(relative, string(filepath.Separator))
	current := absoluteRoot
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			break
		}
		if statErr != nil {
			return "", statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", ErrUnsafeDocumentPath
		}
		if index < len(parts)-1 && !info.IsDir() {
			return "", ErrUnsafeDocumentPath
		}
	}
	return fullPath, nil
}

type treeNode struct {
	node     Node
	children []*treeNode
	order    int
}

func (n *treeNode) find(path string) *treeNode {
	for _, child := range n.children {
		if child.node.Path == path {
			return child
		}
	}
	return nil
}

func sortTree(node *treeNode) {
	for _, child := range node.children {
		sortTree(child)
	}
	sort.SliceStable(node.children, func(i, j int) bool {
		left, right := node.children[i].node, node.children[j].node
		if left.Kind != right.Kind {
			return left.Kind == "category"
		}
		if node.children[i].order != node.children[j].order {
			return node.children[i].order < node.children[j].order
		}
		return strings.ToLower(left.Title) < strings.ToLower(right.Title)
	})
	for _, child := range node.children {
		child.node.Children = make([]Node, 0, len(child.children))
		for _, grandchild := range child.children {
			child.node.Children = append(child.node.Children, grandchild.node)
		}
	}
}

type frontMatter struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Order       int      `yaml:"order"`
	Draft       bool     `yaml:"draft"`
	Tags        []string `yaml:"tags"`
	Slug        string   `yaml:"slug"`
}

type categoryMetadata struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Order       int    `yaml:"order"`
	Collapsed   bool   `yaml:"collapsed"`
}

func readCategoryMetadata(root, categoryPath string) categoryMetadata {
	metadata := categoryMetadata{}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(categoryPath), "_category.yml"))
	if err != nil {
		return metadata
	}
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return categoryMetadata{}
	}
	metadata.Title = strings.TrimSpace(metadata.Title)
	metadata.Description = strings.TrimSpace(metadata.Description)
	return metadata
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func scan(root string) (map[string]Document, []Issue, error) {
	return scanWithDrafts(root, false)
}

func scanWithDrafts(root string, includeDrafts bool) (map[string]Document, []Issue, error) {
	docs := make(map[string]Document)
	var issues []Issue
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		data, err := os.ReadFile(path)
		if err != nil {
			issues = append(issues, Issue{Path: relative, Message: err.Error()})
			return nil
		}
		doc, err := parseDocument(relative, data)
		if err != nil {
			issues = append(issues, Issue{Path: relative, Message: err.Error()})
			return nil
		}
		if includeDrafts || !doc.Draft {
			docs[doc.Path] = doc
		}
		return nil
	})
	return docs, issues, err
}

func parseDocument(relativePath string, data []byte) (Document, error) {
	metadata, body, err := parseFrontMatter(data)
	if err != nil {
		return Document{}, err
	}
	sourcePath := filepath.ToSlash(relativePath)
	path := sourcePath
	if metadata.Slug != "" {
		if clean, cleanErr := cleanDocumentPath(metadata.Slug); cleanErr == nil {
			path = clean
		}
	}
	title := strings.TrimSpace(metadata.Title)
	if title == "" {
		title = firstHeading(body)
	}
	if title == "" {
		title = titleFromFilename(pathpkg.Base(path))
	}
	html, headings := renderMarkdown(body)
	html = rewriteMediaURLs(html)
	sum := sha256.Sum256(data)
	tags := append([]string(nil), metadata.Tags...)
	return Document{
		Path:        path,
		Title:       title,
		Description: strings.TrimSpace(metadata.Description),
		Tags:        tags,
		Hash:        hex.EncodeToString(sum[:]),
		HTML:        html,
		Headings:    headings,
		Draft:       metadata.Draft,
		order:       metadata.Order,
		searchText:  plainText(body),
		sourcePath:  sourcePath,
	}, nil
}

func plainText(body []byte) string {
	htmlLine := regexp.MustCompile(`<[^>]*>`)
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	var output []string
	insideFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			insideFence = !insideFence
			continue
		}
		if insideFence {
			continue
		}
		if strings.Contains(trimmed, "<") && strings.Contains(trimmed, ">") {
			continue
		}
		trimmed = strings.TrimLeft(trimmed, "# >-+*0123456789.\t")
		trimmed = htmlLine.ReplaceAllString(trimmed, " ")
		trimmed = strings.ReplaceAll(trimmed, "![", "[")
		trimmed = strings.ReplaceAll(trimmed, "[", "")
		trimmed = strings.ReplaceAll(trimmed, "]", "")
		trimmed = strings.ReplaceAll(trimmed, "(`", " ")
		trimmed = strings.ReplaceAll(trimmed, "`", "")
		if trimmed != "" {
			output = append(output, trimmed)
		}
	}
	return strings.Join(output, " ")
}

var imageSourceAttribute = regexp.MustCompile(`(?i)(src=")([^"]*)(")`)

func rewriteMediaURLs(html string) string {
	return imageSourceAttribute.ReplaceAllStringFunc(html, func(attribute string) string {
		parts := imageSourceAttribute.FindStringSubmatch(attribute)
		if len(parts) != 4 {
			return attribute
		}
		return parts[1] + mediaURL(parts[2]) + parts[3]
	})
}

func mediaURL(source string) string {
	source = strings.TrimSpace(source)
	lowerSource := strings.ToLower(source)
	if source == "" || strings.HasPrefix(source, "#") || strings.HasPrefix(source, "/") ||
		strings.HasPrefix(lowerSource, "http://") || strings.HasPrefix(lowerSource, "https://") ||
		strings.HasPrefix(lowerSource, "data:image/") {
		return source
	}
	if strings.Contains(source, ":") {
		return ""
	}
	fragment := ""
	if index := strings.IndexAny(source, "?#"); index >= 0 {
		fragment = source[index:]
		source = source[:index]
	}
	clean, err := cleanRelativePath(source)
	if err != nil {
		return source + fragment
	}
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "media/")
	return "/media/" + clean + fragment
}

func snippet(text, query string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	index := strings.Index(lower, query)
	if index < 0 {
		if len([]rune(text)) > 120 {
			return string([]rune(text)[:120]) + "…"
		}
		return text
	}
	runes := []rune(text)
	start := len([]rune(text[:index])) - 40
	if start < 0 {
		start = 0
	}
	end := start + 120
	if end > len(runes) {
		end = len(runes)
	}
	result := string(runes[start:end])
	if start > 0 {
		result = "…" + result
	}
	if end < len(runes) {
		result += "…"
	}
	return result
}

func parseFrontMatter(data []byte) (frontMatter, []byte, error) {
	if !bytes.HasPrefix(data, []byte("---\n")) && !bytes.HasPrefix(data, []byte("---\r\n")) {
		return frontMatter{}, data, nil
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	end := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" || strings.TrimSpace(lines[index]) == "..." {
			end = index
			break
		}
	}
	if end == -1 {
		return frontMatter{}, nil, errors.New("front matter is not closed")
	}
	var metadata frontMatter
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &metadata); err != nil {
		return frontMatter{}, nil, fmt.Errorf("invalid front matter: %w", err)
	}
	return metadata, []byte(strings.Join(lines[end+1:], "\n")), nil
}

func renderMarkdown(body []byte) (string, []Heading) {
	markdown := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	var rendered bytes.Buffer
	if err := markdown.Convert(body, &rendered, parser.WithContext(parser.NewContext(parser.WithIDs(newHeadingIDs())))); err != nil {
		return "", nil
	}
	return rendered.String(), headingsFromMarkdown(body)
}

func headingsFromMarkdown(body []byte) []Heading {
	markdown := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	root := markdown.Parser().Parse(text.NewReader(body), parser.WithContext(parser.NewContext(parser.WithIDs(newHeadingIDs()))))
	var headings []Heading
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node.Kind() != ast.KindHeading {
			return ast.WalkContinue, nil
		}
		heading := node.(*ast.Heading)
		idValue, _ := heading.AttributeString("id")
		id, _ := idValue.([]byte)
		headings = append(headings, Heading{
			Level: heading.Level,
			ID:    string(id),
			Text:  strings.TrimSpace(string(heading.Text(body))),
		})
		return ast.WalkSkipChildren, nil
	})
	return headings
}

type headingIDs struct {
	used map[string]int
}

func newHeadingIDs() parser.IDs {
	return &headingIDs{used: make(map[string]int)}
}

func (ids *headingIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	base := slugify(string(value))
	if base == "" {
		base = "heading"
	}
	ids.used[base]++
	if ids.used[base] > 1 {
		return []byte(fmt.Sprintf("%s-%d", base, ids.used[base]))
	}
	return []byte(base)
}

func (ids *headingIDs) Put(value []byte) {
	ids.used[string(value)]++
}

func firstHeading(body []byte) string {
	for _, heading := range headingsFromMarkdown(body) {
		if heading.Level == 1 {
			return heading.Text
		}
	}
	return ""
}

func slugify(value string) string {
	var builder strings.Builder
	separator := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(r)
			separator = false
			continue
		}
		if builder.Len() > 0 && !separator {
			builder.WriteByte('-')
			separator = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func titleFromFilename(filename string) string {
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))
	filename = strings.ReplaceAll(strings.ReplaceAll(filename, "-", " "), "_", " ")
	return strings.TrimSpace(filename)
}

func cleanDocumentPath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.ContainsRune(value, 0) || strings.HasPrefix(value, "/") {
		return "", errors.New("invalid document path")
	}
	clean := pathpkg.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", errors.New("document path escapes content root")
	}
	ext := strings.ToLower(pathpkg.Ext(clean))
	if ext != ".md" && ext != ".markdown" {
		return "", errors.New("document path must reference markdown")
	}
	return clean, nil
}

func cleanCategoryPath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.ContainsRune(value, 0) || strings.HasPrefix(value, "/") {
		return "", ErrUnsafeCategoryPath
	}
	clean := pathpkg.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", ErrUnsafeCategoryPath
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") {
			return "", ErrUnsafeCategoryPath
		}
	}
	return clean, nil
}

func resolveCategoryPath(root, relativePath string) (string, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(absoluteRoot, filepath.FromSlash(relativePath))
	relative, err := filepath.Rel(absoluteRoot, fullPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrUnsafeCategoryPath
	}
	current := absoluteRoot
	parts := strings.Split(relative, string(filepath.Separator))
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			break
		}
		if statErr != nil {
			return "", statErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", ErrUnsafeCategoryPath
		}
	}
	return fullPath, nil
}
