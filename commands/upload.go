package commands

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"regexp"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/go-yaml/yaml"
	"github.com/p47t/md2cfl/bf2confluence"
	"github.com/p47t/md2cfl/confluence"
	"github.com/p47t/md2cfl/parser/pageparser"
	"github.com/russross/blackfriday/v2"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/ssh/terminal"
)

type uploadCmd struct {
	*cobra.Command
	pageId string
	title  string
	dryrun bool
}

func newUploadCmd() *cobra.Command {
	var c uploadCmd
	c.Command = &cobra.Command{
		Use:   "upload [file]",
		Short: "Upload file to Confluence page",
		Long: `Upload converted markdown file to specified Confluence page.

Note that you may put Confluence-related parameters in front matter, e.g.:

---
confluence:
	base: "http://your.confluence.server"
	page: "583910399"
---
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse markdown
			var pmd parsedMarkdown
			err := pmd.parse(args[0])
			if err != nil {
				return err
			}

			// Connect to Wiki
			baseUrl := pmd.ConfluenceBase(rootCmd.baseUrl)
			log.Println("Confluence Base:", baseUrl)
			auth, err := getConfluenceAuth(baseUrl)
			if err != nil {
				return err
			}
			wiki, err := confluence.NewWiki(baseUrl, auth)
			if err != nil {
				return err
			}

			pageId := pmd.ConfluencePage(c.pageId)
			wikiText := pmd.render()

			if c.dryrun {
				fmt.Print(string(wikiText))
				return nil
			}

			// Upload page
			webUI, err := uploadPage(wiki, pageId, pmd.ConfluenceFormat("wiki"), wikiText, pmd.Title(c.title))
			if err != nil {
				return err
			}

			// Upload attachments
			mdPath := path.Dir(args[0])
			var attachments []string
			for _, dest := range append(pmd.images(), pmd.links()...) {
				if _, err := url.ParseRequestURI(dest); err == nil {
					continue // ignore all remote or absolute path
				}
				attachments = append(attachments, path.Join(mdPath, dest))
			}
			_, errs := wiki.AddUpdateAttachments(pageId, attachments, func(msg string) {
				log.Println(msg)
			})
			for _, err := range errs {
				log.Println(err) // log but don't report error to caller
			}

			// Update labels
			if labels := pmd.Tags(); len(labels) > 0 {
				log.Println("Updating labels...")
				err = wiki.AddLabels(pageId, labels)
				if err != nil {
					return err
				}
			}

			log.Println("File is uploaded successfully.")
			if webUI != "" {
				log.Println("Browse", baseUrl+webUI, "for the result.")
			}
			return nil
		},
	}
	c.Command.Flags().StringVarP(&c.pageId, "page", "P", "", "page ID")
	c.Command.Flags().StringVarP(&c.title, "title", "t", "Page", "page title")
	c.Command.Flags().BoolVarP(&c.dryrun, "dryrun", "d", false, "don't upload but print wiki text")

	return c.Command
}

func getConfluenceAuth(baseUrl string) (confluence.AuthMethod, error) {
	var err error
	userName, password := rootCmd.userName, rootCmd.password
	if userName == "" {
		return nil, fmt.Errorf("must specify user name")
	}
	if password == "" && rootCmd.useSavedCredential {
		// Load credential from system key ring
		password, _ = keyring.Get(baseUrl, userName)
	}
	if password == "" {
		fmt.Printf("Password for %v at %v: ", userName, baseUrl)
		if bytePassword, err := terminal.ReadPassword(int(syscall.Stdin)); err != nil {
			return nil, err
		} else {
			password = string(bytePassword)
			fmt.Println()
		}
	}
	if rootCmd.saveCredential {
		// Save credential to system key ring
		if err = keyring.Set(baseUrl, userName, password); err != nil {
			log.Fatal(err)
		}
	}

	return confluence.BasicAuth(userName, password), nil
}

type parsedMarkdown struct {
	frontMatterSource []byte
	frontMatter       map[string]interface{}

	// Everything after Front Matter
	content    []byte
	contentAst *blackfriday.Node
}

func (pf *parsedMarkdown) Title(def string) string {
	if title, ok := pf.frontMatter["title"]; ok {
		return title.(string)
	}
	return def
}

func (pf *parsedMarkdown) ConfluenceBase(def string) string {
	if cfl, ok := pf.frontMatter["confluence"]; ok {
		if base, ok := cfl.(map[interface{}]interface{})["base"]; ok {
			return base.(string)
		}
	}
	return def
}

func (pf *parsedMarkdown) ConfluencePage(def string) string {
	if cfl, ok := pf.frontMatter["confluence"]; ok {
		if page, ok := cfl.(map[interface{}]interface{})["page"]; ok {
			return page.(string)
		}
	}
	return def
}

func (pf *parsedMarkdown) ConfluenceFormat(def string) string {
	if cfl, ok := pf.frontMatter["confluence"]; ok {
		if format, ok := cfl.(map[interface{}]interface{})["format"]; ok {
			return format.(string)
		}
	}
	return def
}

func (pf *parsedMarkdown) Tags() []string {
	var ret []string
	if tags, ok := pf.frontMatter["tags"]; ok {
		for _, t := range tags.([]interface{}) {
			ret = append(ret, t.(string))
		}
	}
	return ret
}

var (
	reShortcode = regexp.MustCompile(`{{%\s*/?(\S+)\s*%}}`)
)

func (pf *parsedMarkdown) parse(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	psr, err := pageparser.Parse(f)
	if err != nil {
		return err
	}

	var frontMatterError = fmt.Errorf("no front matter is provided")
	psr.Iterator().PeekWalk(func(item pageparser.Item) bool {
		if pf.frontMatterSource != nil {
			pf.content = psr.Input()[item.Pos:]
			return false
		} else if item.IsFrontMatter() {
			pf.frontMatterSource = item.Val

			// Try YAML
			if err := yaml.Unmarshal(pf.frontMatterSource, &pf.frontMatter); err != nil {
				// Try TOML
				if err := toml.Unmarshal(pf.frontMatterSource, &pf.frontMatter); err != nil {
					// TODO: support more format?
					frontMatterError = fmt.Errorf("invalid front matter")
					return false
				}
			}
			frontMatterError = nil
		}
		return true
	})
	if frontMatterError != nil {
		return frontMatterError
	}

	// Remove Hugo shortcode "{{% note %}} ... {{% /note %}}"
	pf.content = reShortcode.ReplaceAll(pf.content, []byte(``))

	extensions := blackfriday.CommonExtensions & ^blackfriday.Autolink
	bf := blackfriday.New(blackfriday.WithExtensions(extensions))
	pf.contentAst = bf.Parse(pf.content)

	return nil
}

func (pf *parsedMarkdown) render() []byte {
	if pf.ConfluenceFormat("wiki") == "xml" {
		renderer := &bf2confluence.XmlRenderer{
			Renderer: bf2confluence.Renderer{Flags: bf2confluence.InformationMacros | bf2confluence.RawConfluenceWiki}}
		return renderer.Render(pf.contentAst)
	}
	renderer := &bf2confluence.Renderer{Flags: bf2confluence.InformationMacros | bf2confluence.RawConfluenceWiki}
	return renderer.Render(pf.contentAst)
}

func (pf *parsedMarkdown) images() []string {
	return pf.destinations(blackfriday.Image)
}

func (pf *parsedMarkdown) links() []string {
	return pf.destinations(blackfriday.Link)
}

func (pf *parsedMarkdown) destinations(t blackfriday.NodeType) []string {
	var destinations []string
	pf.contentAst.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if node.Type == t && entering {
			destinations = append(destinations, string(node.LinkData.Destination))
		}
		return blackfriday.GoToNext
	})
	return destinations
}

func uploadPage(wiki *confluence.Wiki, pageId, format string, content []byte, title string) (string, error) {
	log.Println("Confluence Page:", pageId)

	page, err := preparePage(wiki, pageId, format, content, title)
	if err != nil {
		return "", err
	}

	_, err = wiki.UpdateContent(page)
	return page.Links.WebUI, err
}

func preparePage(wiki *confluence.Wiki, pageId, format string, content []byte, title string) (*confluence.Content, error) {
	page, err := wiki.GetContent(pageId, []string{"body", "version"})
	if err != nil {
		return nil, err
	}

	if title != "" {
		page.Title = title
	}
	page.Body.Storage.Value = string(content)
	switch format {
	case "xml":
		page.Body.Storage.Representation = "storage"
	case "wiki":
		fallthrough
	default:
		page.Body.Storage.Representation = "wiki"
	}
	page.Version.Number += 1

	return page, nil
}
