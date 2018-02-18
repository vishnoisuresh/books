package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/essentialbooks/books/pkg/common"
	"github.com/essentialbooks/books/pkg/kvstore"
	"github.com/kjk/u"
)

const (
	fullURLBase = "https://www.programming-books.io"
)

// Article represents a part of a chapter
type Article struct {
	// stable, globally unique (across all bookd) id
	// either imported Id from Stack Overflow or auto-generated by us
	// allows stable urls and being able to cross-reference articles
	ID           string
	No           int      // TODO: can I get rid of this?
	Chapter      *Chapter // reference to containing chapter
	Title        string   // used in book_index.tmpl.html
	FileNameBase string   // base for both filename and url, format: a-${ID}-${Title}
	BodyMarkdown string
	// TODO: we should convert all HTML content to markdown
	BodyHTML template.HTML

	// for generating toc of a chapter, all articles that belong to the same
	// chapter as this article
	Siblings  []Article
	IsCurrent bool // only used when part of Siblings

	sourceFilePath string // path of the file from which we've read the article
	AnalyticsCode  string
}

// Book retuns book this article belongs to
func (a *Article) Book() *Book {
	return a.Chapter.Book
}

// URL returns url of .html file with this article
func (a *Article) URL() string {
	chap := a.Chapter
	book := chap.Book
	// /essential/go/a-14047-flags
	return fmt.Sprintf("/essential/%s/%s", book.FileNameBase, a.FileNameBase)
}

// CanonnicalURL returns full url including host
func (a *Article) CanonnicalURL() string {
	return fullURLBase + a.URL()
}

// GitHubText returns text we display in GitHub box
func (a *Article) GitHubText() string {
	return "Edit on GitHub"
}

// GitHubURL returns url to GitHub repo
func (a *Article) GitHubURL() string {
	uri := a.Chapter.GitHubURL() + "/" + filepath.Base(a.sourceFilePath)
	uri = strings.Replace(uri, "/tree/", "/blob/", -1)
	return uri
}

// GitHubEditURL returns url to editing this article on GitHub
// same as GitHubURL because we don't want to automatically fork
// the repo as would happen if we used /edit/ url
func (a *Article) GitHubEditURL() string {
	return a.GitHubURL()
}

// GitHubIssueURL returns link for reporting an issue about an article on githbu
// https://github.com/essentialbooks/books/issues/new?title=${title}&body=${body}&labels=docs"
func (a *Article) GitHubIssueURL() string {
	title := fmt.Sprintf("Issue for article '%s'", a.Title)
	body := fmt.Sprintf("From URL: %s\nFile: %s\n", a.CanonnicalURL(), a.GitHubEditURL())
	return gitHubBaseURL + fmt.Sprintf("/issues/new?title=%s&body=%s&labels=docs", title, body)
}

func (a *Article) destFilePath() string {
	return filepath.Join(destEssentialDir, a.Book().FileNameBase, a.FileNameBase+".html")
}

// Chapter represents a book chapter
type Chapter struct {
	// stable, globally unique (across all bookd) id
	// either imported Id from Stack Overflow or auto-generated by us
	// allows stable urls and being able to cross-reference articles
	ID         string
	Book       *Book
	ChapterDir string
	// full path to 000-index.md file
	indexFilePath string
	indexDoc      kvstore.Doc // content of 000-index.md file
	Title         string      // extracted from IndexKV, used in book_index.tmpl.html
	FileNameBase  string      // format: ch-${ID}-${Title}, used for URL and .html file name
	Articles      []*Article
	No            int

	AnalyticsCode string
}

// URL is used in book_index.tmpl.html
func (c *Chapter) URL() string {
	// /essential/go/ch-4023-parsing-command-line-arguments-and-flags
	return fmt.Sprintf("/essential/%s/%s", c.Book.FileNameBase, c.FileNameBase)
}

// CanonnicalURL returns full url including host
func (c *Chapter) CanonnicalURL() string {
	return fullURLBase + c.URL()
}

// GitHubText returns text we display in GitHub box
func (c *Chapter) GitHubText() string {
	return "Edit on GitHub"
}

// GitHubURL returns url to GitHub repo
func (c *Chapter) GitHubURL() string {
	return c.Book.GitHubURL() + "/" + c.ChapterDir
}

// GitHubEditURL returns url to edit 000-index.md document
func (c *Chapter) GitHubEditURL() string {
	bookDir := filepath.Base(c.Book.destDir)
	uri := gitHubBaseURL + "/blob/master/books/" + bookDir
	return uri + "/" + c.ChapterDir + "/000-index.md"
}

// GitHubIssueURL returns link for reporting an issue about an article on githbu
// https://github.com/essentialbooks/books/issues/new?title=${title}&body=${body}&labels=docs"
func (c *Chapter) GitHubIssueURL() string {
	title := fmt.Sprintf("Issue for chapter '%s'", c.Title)
	body := fmt.Sprintf("From URL: %s\nFile: %s\n", c.CanonnicalURL(), c.GitHubEditURL())
	return gitHubBaseURL + fmt.Sprintf("/issues/new?title=%s&body=%s&labels=docs", title, body)
}

func (c *Chapter) destFilePath() string {
	return filepath.Join(destEssentialDir, c.Book.FileNameBase, c.FileNameBase+".html")
}

// VersionsHTML returns html version of versions
func (c *Chapter) VersionsHTML() template.HTML {
	s, err := c.indexDoc.GetValue("VersionsHtml")
	if err != nil {
		s = ""
	}
	return template.HTML(s)
}

// TODO: get rid of IntroductionHTML, SyntaxHTML etc., convert to just Body in markdown format

// BodyHTML retruns html version of Body: field
func (c *Chapter) BodyHTML() template.HTML {
	s, err := c.indexDoc.GetValue("Body")
	if err != nil {
		return template.HTML("")
	}
	html := markdownToHTML([]byte(s), "", c.Book)
	return template.HTML(html)
}

// IntroductionHTML retruns html version of Introduction:
func (c *Chapter) IntroductionHTML() template.HTML {
	s, err := c.indexDoc.GetValue("Introduction")
	if err != nil {
		return template.HTML("")
	}
	html := markdownToHTML([]byte(s), "", c.Book)
	return template.HTML(html)
}

// SyntaxHTML retruns html version of Syntax:
func (c *Chapter) SyntaxHTML() template.HTML {
	s, err := c.indexDoc.GetValue("Syntax")
	if err != nil {
		return template.HTML("")
	}
	html := markdownToHTML([]byte(s), "", c.Book)
	return template.HTML(html)
}

// RemarksHTML retruns html version of Remarks:
func (c *Chapter) RemarksHTML() template.HTML {
	s, err := c.indexDoc.GetValue("Remarks")
	if err != nil {
		return template.HTML("")
	}
	html := markdownToHTML([]byte(s), "", c.Book)
	return template.HTML(html)
}

// ContributorsHTML retruns html version of Contributors:
func (c *Chapter) ContributorsHTML() template.HTML {
	s, err := c.indexDoc.GetValue("Contributors")
	if err != nil {
		return template.HTML("")
	}
	html := markdownToHTML([]byte(s), "", c.Book)
	return template.HTML(html)
}

// SoContributor describes a StackOverflow contributor
type SoContributor struct {
	ID      int
	URLPart string
	Name    string
}

// Book represents a book
type Book struct {
	Title          string // used in index.tmpl.html
	titleSafe      string
	TitleLong      string // used in book_index.tmpl.html
	FileNameBase   string
	Chapters       []*Chapter
	sourceDir      string // dir where source markdown files are
	destDir        string // dif where destitation html files are
	SoContributors []SoContributor

	cachedArticlesCount int
	defaultLang         string // default programming language for programming examples
	knownUrls           []string

	AnalyticsCode string

	// for concurrency
	sem chan bool
	wg  sync.WaitGroup
}

// ContributorCount returns number of contributors
func (b *Book) ContributorCount() int {
	return len(b.SoContributors)
}

// ContributorsURL returns url of the chapter that lists contributors
func (b *Book) ContributorsURL() string {
	return b.URL() + "/ch-contributors"
}

// GitHubText returns text we show in GitHub link
func (b *Book) GitHubText() string {
	return "Edit on GitHub"
}

// GitHubURL returns link to GitHub for this book
func (b *Book) GitHubURL() string {
	return gitHubBaseURL + "/tree/master/books/" + filepath.Base(b.destDir)
}

// URL returns url of the book, used in index.tmpl.html
func (b *Book) URL() string {
	return fmt.Sprintf("/essential/%s/", b.titleSafe)
}

// CanonnicalURL returns full url including host
func (b *Book) CanonnicalURL() string {
	return fullURLBase + b.URL()
}

// ShareOnTwitterText returns text for sharing on twitter
func (b *Book) ShareOnTwitterText() string {
	return fmt.Sprintf(`"Essential %s" - a free programming book`, b.Title)
}

// TocSearchJSURL returns data for searching titles of chapters/articles
func (b *Book) TocSearchJSURL() string {
	return b.URL() + "/toc_search.js"
}

// CoverURL returns url to cover image
func (b *Book) CoverURL() string {
	coverName := langToCover[b.titleSafe]
	return fmt.Sprintf("/covers/%s.png", coverName)
}

// CoverFullURL returns a URL for the cover including host
func (b *Book) CoverFullURL() string {
	return fullURLBase + b.CoverURL()
}

// CoverTwitterFullURL returns a URL for the cover including host
func (b *Book) CoverTwitterFullURL() string {
	coverName := langToCover[b.titleSafe]
	return fullURLBase + fmt.Sprintf("/covers/twitter/%s.png", coverName)
}

// ArticlesCount returns total number of articles
func (b *Book) ArticlesCount() int {
	if b.cachedArticlesCount != 0 {
		return b.cachedArticlesCount
	}
	n := 0
	for _, ch := range b.Chapters {
		n += len(ch.Articles)
	}
	// each chapter has 000-index.md which is also an article
	n += len(b.Chapters)
	b.cachedArticlesCount = n
	return n
}

// ChaptersCount returns number of chapters
func (b *Book) ChaptersCount() int {
	return len(b.Chapters)
}

var (
	defTitle = "No Title"
)

func dumpKV(doc kvstore.Doc) {
	for _, kv := range doc {
		fmt.Printf("K: %s\nV: %s\n", kv.Key, common.ShortenString(kv.Value))
	}
}

func parseArticle(path string) (*Article, error) {
	doc, err := paarseKVFileWithIncludes(path)
	if err != nil {
		fmt.Printf("Error parsing KV file: '%s'\n", path)
		maybePanicIfErr(err)
		return nil, err
	}
	article := &Article{
		sourceFilePath: path,
	}
	article.ID, err = doc.GetValue("Id")
	if err != nil {
		return nil, fmt.Errorf("parseArticle('%s'), err: '%s'", path, err)
	}
	if strings.Contains(article.ID, " ") {
		return nil, fmt.Errorf("parseArticle('%s'), res.ID = '%s' has space in it", path, article.ID)
	}
	article.Title = doc.GetValueSilent("Title", defTitle)
	if article.Title == defTitle {
		fmt.Printf("parseArticle: no title for %s\n", path)
	}
	titleSafe := common.MakeURLSafe(article.Title)
	article.FileNameBase = fmt.Sprintf("a-%s-%s", article.ID, titleSafe)
	article.BodyMarkdown, err = doc.GetValue("Body")
	if err == nil {
		return article, nil
	}
	s, err := doc.GetValue("BodyHtml")
	article.BodyHTML = template.HTML(s)
	if err == nil {
		return article, nil
	}
	// on parsing error, dump the doc
	dumpKV(doc)
	return nil, fmt.Errorf("parseArticle('%s'), err: '%s'", path, err)
}

func buildArticleSiblings(articles []*Article) {
	// build a template
	var siblings []Article
	for i, article := range articles {
		sibling := *article // making a copy, we can't touch the original
		sibling.No = i + 1
		siblings = append(siblings, sibling)
	}
	// for each article, copy a template and set IsCurrent
	for i, article := range articles {
		copy := append([]Article(nil), siblings...)
		copy[i].IsCurrent = true
		article.Siblings = copy
	}
}

// Parses @file ${fileName} directives and replaces them
// with the content of the file
func processFileIncludes(path string) ([]string, error) {
	lines, err := common.ReadFileAsLines(path)
	if err != nil {
		return nil, err
	}
	var res []string
	for _, line := range lines {
		if strings.HasPrefix(line, "@file ") {
			//fmt.Printf("processFileIncludes('%s'\n", path)
			lines2, err := extractCodeSnippetsAsMarkdownLines(filepath.Dir(path), line)
			if err != nil {
				fmt.Printf("processFileIncludes: error '%s'\n", err)
				return nil, err
			}
			res = append(res, lines2...)
		} else {
			res = append(res, line)
		}
	}
	return res, nil
}

func paarseKVFileWithIncludes(path string) (kvstore.Doc, error) {
	lines, err := processFileIncludes(path)
	if err == nil {
		return kvstore.ParseKVLines(lines)
	}
	// if processFileIncludes fails we retry without file includes
	return kvstore.ParseKVFile(path)
}

func parseChapter(chapter *Chapter) error {
	dir := filepath.Join(chapter.Book.sourceDir, chapter.ChapterDir)
	path := filepath.Join(dir, "000-index.md")
	chapter.indexFilePath = path
	doc, err := paarseKVFileWithIncludes(path)
	if err != nil {
		fmt.Printf("Error parsing KV file: '%s'\n", path)
		maybePanicIfErr(err)
	}

	chapter.indexDoc = doc
	chapter.Title, err = doc.GetValue("Title")
	if err != nil {
		return fmt.Errorf("parseChapter('%s'), missing Title, err: '%s'", path, err)
	}
	chapter.ID, err = doc.GetValue("Id")
	if err != nil {
		return fmt.Errorf("parseChapter('%s'), missing Id, err: '%s'", path, err)
	}

	if strings.Contains(chapter.ID, " ") {
		return fmt.Errorf("parseChapter('%s'), chapter.ID = '%s' has space in it", path, chapter.ID)
	}

	titleSafe := common.MakeURLSafe(chapter.Title)
	chapter.FileNameBase = fmt.Sprintf("ch-%s-%s", chapter.ID, titleSafe)
	fileInfos, err := ioutil.ReadDir(dir)
	var articles []*Article
	for _, fi := range fileInfos {
		if fi.IsDir() || !fi.Mode().IsRegular() {
			continue
		}
		name := fi.Name()
		if strings.ToLower(filepath.Ext(name)) != ".md" {
			continue
		}

		// some files are not meant to be processed here
		switch strings.ToLower(name) {
		case "000-index.md":
			continue
		}
		path = filepath.Join(dir, name)
		article, err := parseArticle(path)
		if err != nil {
			return err
		}
		article.Chapter = chapter
		article.No = len(articles) + 1
		articles = append(articles, article)
	}
	buildArticleSiblings(articles)
	chapter.Articles = articles
	return nil
}

func soContributorURL(userID int, userName string) string {
	return fmt.Sprintf("https://stackoverflow.com/users/%d/%s", userID, userName)
}

func loadSoContributorsMust(book *Book, path string) {
	lines, err := common.ReadFileAsLines(path)
	u.PanicIfErr(err)
	var contributors []SoContributor
	for _, line := range lines {
		id, err := strconv.Atoi(line)
		u.PanicIfErr(err)
		name := soUserIDToNameMap[id]
		u.PanicIf(name == "", "no SO contributor for id %d", id)
		if name == "user_deleted" {
			continue
		}
		nameUnescaped, err := url.PathUnescape(name)
		u.PanicIfErr(err)
		c := SoContributor{
			ID:      id,
			URLPart: name,
			Name:    nameUnescaped,
		}
		contributors = append(contributors, c)
	}
	sort.Slice(contributors, func(i, j int) bool {
		return contributors[i].Name < contributors[j].Name
	})
	book.SoContributors = contributors
}

// TODO: add github contributors
func genContributorsMarkdown(contributors []SoContributor) string {
	if len(contributors) == 0 {
		return ""
	}
	lines := []string{
		"Contributors from [GitHub](https://github.com/essentialbooks/books/graphs/contributors)",
		"",
		"Contributors from Stack Overflow:",
	}
	for _, c := range contributors {
		s := fmt.Sprintf("* [%s](%s)", c.Name, soContributorURL(c.ID, c.Name))
		lines = append(lines, s)
	}
	return strings.Join(lines, "\n")
}

func genContributorsChapter(book *Book) *Chapter {
	md := genContributorsMarkdown(book.SoContributors)
	var doc kvstore.Doc
	kv := kvstore.KeyValue{
		Key:   "Body",
		Value: md,
	}
	doc = append(doc, kv)
	ch := &Chapter{
		Book:         book,
		indexDoc:     doc,
		Title:        "Contributors",
		FileNameBase: "ch-contributors",
		No:           999,
	}
	return ch
}

// make sure chapter/article ids within the book are unique,
// so that we can generate stable urls.
// also build a list of chapter/article urls
func ensureUniqueIds(book *Book) {
	var urls []string
	chapterIds := make(map[string]*Chapter)
	articleIds := make(map[string]*Article)
	for _, c := range book.Chapters {
		if chap, ok := chapterIds[c.ID]; ok {
			fmt.Printf("Duplicate chapter id '%s' in:\n", c.ID)
			fmt.Printf("Chapter '%s', file: '%s'\n", c.Title, c.indexFilePath)
			fmt.Printf("Chapter '%s', file: '%s'\n", chap.Title, chap.indexFilePath)
			os.Exit(1)
		}
		chapterIds[c.ID] = c
		urls = append(urls, c.FileNameBase)
		for _, a := range c.Articles {
			if a2, ok := articleIds[a.ID]; ok {
				err := fmt.Errorf("Duplicate article id: '%s', in: %s and %s", a.ID, a.sourceFilePath, a2.sourceFilePath)
				maybePanicIfErr(err)
			} else {
				articleIds[a.ID] = a
				urls = append(urls, a.FileNameBase)
			}
		}
	}
	book.knownUrls = urls
}

func parseBook(bookName string) (*Book, error) {
	timeStart := time.Now()
	fmt.Printf("Parsing book %s\n", bookName)
	bookNameSafe := common.MakeURLSafe(bookName)
	srcDir := filepath.Join("books", bookNameSafe)
	book := &Book{
		Title:        bookName,
		titleSafe:    bookNameSafe,
		TitleLong:    fmt.Sprintf("Essential %s", bookName),
		FileNameBase: bookNameSafe,
		sourceDir:    srcDir,
		destDir:      filepath.Join(destEssentialDir, bookNameSafe),
	}

	fileInfos, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}

	nProcs := getAlmostMaxProcs()

	sem := make(chan bool, nProcs)
	var wg sync.WaitGroup
	var chapters []*Chapter
	var err2 error

	for _, fi := range fileInfos {
		if fi.IsDir() {
			ch := &Chapter{
				Book:       book,
				ChapterDir: fi.Name(),
			}
			chapters = append(chapters, ch)
			sem <- true
			wg.Add(1)
			go func(chap *Chapter) {
				err = parseChapter(chap)
				if err != nil {
					// not thread safe but whatever
					err2 = err
				}
				<-sem
				wg.Done()
			}(ch)
			continue
		}

		name := strings.ToLower(fi.Name())
		// some files should be ignored
		if name == "toc.txt" {
			continue
		}
		if name == "so_contributors.txt" {
			path := filepath.Join(srcDir, fi.Name())
			loadSoContributorsMust(book, path)
			continue
		}
		return nil, fmt.Errorf("Unexpected file at top-level: '%s'", fi.Name())
	}
	wg.Wait()

	ch := genContributorsChapter(book)
	chapters = append(chapters, ch)

	for i, ch := range chapters {
		ch.No = i + 1
	}
	book.Chapters = chapters

	ensureUniqueIds(book)

	fmt.Printf("Book '%s' %d chapters, %d articles, finished parsing in %s\n", bookName, len(chapters), book.ArticlesCount(), time.Since(timeStart))
	return book, err2
}
