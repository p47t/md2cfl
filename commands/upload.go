package commands

import (
	"github.com/BurntSushi/toml"
	"github.com/go-yaml/yaml"
	"github.com/p47t/md2cfl/bf2confluence"
	"github.com/p47t/md2cfl/parser/pageparser"
	"github.com/russross/blackfriday/v2"
	"github.com/seppestas/go-confluence"
	"github.com/spf13/cobra"
	"log"
	"os"
)

type uploadCmd struct {
	*cobra.Command
	pageId string
	title  string
}

func newUploadCmd() *cobra.Command {
	var c uploadCmd
	c.Command = &cobra.Command{
		Use:   "upload [file]",
		Short: "Upload file to Confluence page",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			var pmd parsedMarkdown
			if err = pmd.parse(args[0]); err != nil {
				return err
			}

			renderer := &bf2confluence.Renderer{}
			extensions := blackfriday.CommonExtensions
			bf := blackfriday.New(
				blackfriday.WithRenderer(renderer),
				blackfriday.WithExtensions(extensions))
			ast := bf.Parse(pmd.content)

			var images []string
			ast.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
				switch node.Type {
				case blackfriday.Image:
					if entering {
						images = append(images, string(node.LinkData.Destination))
					}
				}
				return blackfriday.GoToNext
			})

			return uploadPage(
				pmd.ConfluenceBase(rootCmd.baseUrl),
				pmd.ConfluencePage(c.pageId),
				renderer.Render(ast),
				pmd.Title(c.title))
		},
	}
	c.Command.Flags().StringVarP(&c.pageId, "page", "P", "", "page ID")
	c.Command.Flags().StringVarP(&c.title, "title", "t", "Page", "page title")
	return c.Command
}

type parsedMarkdown struct {
	frontMatterSource []byte
	frontMatter       map[string]interface{}

	// Everything after Front Matter
	content []byte
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

func (pf *parsedMarkdown) parse(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	var psr pageparser.Result
	psr, err = pageparser.Parse(f)
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

	return nil
}

func uploadPage(baseUrl string, pageId string, content []byte, title string) error {
	var err error
	var wiki *confluence.Wiki
	var page *confluence.Content

	log.Println("Confluence Base:", baseUrl)
	log.Println("Confluence Page:", pageId)

	if wiki, err = confluence.NewWiki(baseUrl, confluence.BasicAuth(rootCmd.userName, rootCmd.password)); err != nil {
		return err
	}

	if page, err = preparePage(wiki, pageId, content, title); err != nil {
		return err
	}

	if _, err := wiki.UpdateContent(page); err != nil {
		return err
	}

	// TODO: upload attachments
	// curl -v -S -u admin:admin -X POST -H "X-Atlassian-Token: no-check" -F "file=@myfile.txt" -F
	//"comment=this is my file" "http://localhost:8080/confluence/rest/api/content/3604482/child/attachment"

	return nil
}

func preparePage(wiki *confluence.Wiki, pageId string, content []byte, title string) (*confluence.Content, error) {
	var err error
	var page *confluence.Content
	if page, err = wiki.GetContent(pageId, []string{"body", "version"}); err != nil {
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
