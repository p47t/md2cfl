package commands

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/go-yaml/yaml"
	"github.com/p47t/md2cfl/bf2confluence"
	"github.com/p47t/md2cfl/confluence"
	"github.com/p47t/md2cfl/parser/pageparser"
	"github.com/russross/blackfriday/v2"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"regexp"
)

type uploadCmd struct {
	*cobra.Command
	pageId string
	title  string
	output string
}

func newUploadCmd() *cobra.Command {
	var c uploadCmd
	c.Command = &cobra.Command{
		Use:   "upload [file]",
		Short: "Upload file to Confluence page",
		Args:  cobra.MinimumNArgs(1),
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

			// Upload page
			pageId := pmd.ConfluencePage(c.pageId)
			wikiText := pmd.render()
			err = uploadPage(wiki, pageId, wikiText, pmd.Title(c.title))
			if err != nil {
				return err
			}
			if c.output != "" {
				ioutil.WriteFile(c.output, wikiText, 0644)
			}

			// Upload images
			mdPath := path.Dir(args[0])
			var images []string
			for _, im := range pmd.images() {
				if _, err := url.ParseRequestURI(im); err == nil {
					continue // ignore all remote or absolute path
				}
				images = append(images, path.Join(mdPath, im))
			}
			_, errs := wiki.AddUpdateAttachments(pageId, images)
			for _, err := range errs {
				log.Println(err) // log but don't report error to caller
			}

			log.Println("File is uploaded successfully.")
			return nil
		},
	}
	c.Command.Flags().StringVarP(&c.pageId, "page", "P", "", "page ID")
	c.Command.Flags().StringVarP(&c.title, "title", "t", "Page", "page title")
	c.Command.Flags().StringVarP(&c.output, "output", "o", "", "output converted wiki to file")

	return c.Command
}

func getConfluenceAuth(baseUrl string) (confluence.AuthMethod, error) {
	var err error
	userName, password := rootCmd.userName, rootCmd.password
	if userName == "" {
		return nil, fmt.Errorf("must specify user name")
	} else if password == "" {
		// Load credential from system key ring
		if password, err = keyring.Get(baseUrl, userName); err != nil {
			return nil, err
		}
	} else if rootCmd.saveCredential {
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

	psr.Iterator().PeekWalk(func(item pageparser.Item) bool {
		if pf.frontMatterSource != nil {
			pf.content = psr.Input()[item.Pos:]
			return false
		} else if item.IsFrontMatter() {
			pf.frontMatterSource = item.Val

			// TODO: support more formats?
			if err := yaml.Unmarshal(pf.frontMatterSource, &pf.frontMatter); err != nil {
				toml.Unmarshal(pf.frontMatterSource, &pf.frontMatter)
			}
		}
		return true
	})

	// Remove Hugo shortcode "{{% note %}} ... {{% /note %}}"
	pf.content = reShortcode.ReplaceAll(pf.content, []byte(``))

	extensions := blackfriday.CommonExtensions
	bf := blackfriday.New(blackfriday.WithExtensions(extensions))
	pf.contentAst = bf.Parse(pf.content)

	return nil
}

func (pf *parsedMarkdown) render() []byte {
	renderer := &bf2confluence.Renderer{}
	return renderer.Render(pf.contentAst)
}

func (pf *parsedMarkdown) images() []string {
	var images []string
	pf.contentAst.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		switch node.Type {
		case blackfriday.Image:
			if entering {
				images = append(images, string(node.LinkData.Destination))
			}
		}
		return blackfriday.GoToNext
	})
	return images
}

func uploadPage(wiki *confluence.Wiki, pageId string, content []byte, title string) error {
	log.Println("Confluence Page:", pageId)

	page, err := preparePage(wiki, pageId, content, title)
	if err != nil {
		return err
	}

	_, err = wiki.UpdateContent(page)
	return err
}

func preparePage(wiki *confluence.Wiki, pageId string, content []byte, title string) (*confluence.Content, error) {
	page, err := wiki.GetContent(pageId, []string{"body", "version"})
	if err != nil {
		return nil, err
	}

	if title != "" {
		page.Title = title
	}
	page.Body.Storage.Value = string(content)
	page.Body.Storage.Representation = "wiki"
	page.Version.Number += 1

	return page, nil
}
