package commands

import (
	"github.com/p47t/md2cfl/bf2confluence"
	"github.com/p47t/md2cfl/parser/pageparser"
	"github.com/russross/blackfriday/v2"
	"github.com/seppestas/go-confluence"
	"github.com/spf13/cobra"

	"os"
)

var cmdUpload = &cobra.Command{
	Use:   "upload [file] [page ID]",
	Short: "Upload file to Confluence page",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		var output []byte
		if output, err = convertFile(args[0]); err != nil {
			return err
		}
		return uploadPage(output, args[1])
	},
}

type parsedFile struct {
	frontMatterSource []byte
	frontMatter       map[string]interface{}

	// Everything after Front Matter
	content []byte
}

func convertFile(fname string) ([]byte, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var psr pageparser.Result
	psr, err = pageparser.Parse(f)
	if err != nil {
		return nil, err
	}

	var pf parsedFile
	psr.Iterator().PeekWalk(func(item pageparser.Item) bool {
		if pf.frontMatterSource != nil {
			pf.content = psr.Input()[item.Pos:]
			return false
		} else if item.IsFrontMatter() {
			pf.frontMatterSource = item.Val
		}
		return true
	})

	// TODO: get Confluence endpoint and page ID from front matter

	renderer := &bf2confluence.Renderer{}
	extensions := blackfriday.CommonExtensions
	md := blackfriday.New(
		blackfriday.WithRenderer(renderer),
		blackfriday.WithExtensions(extensions))
	ast := md.Parse(pf.content)
	return renderer.Render(ast), nil
}

func uploadPage(content []byte, pageId string) error {
	var err error
	var wiki *confluence.Wiki
	var page *confluence.Content

	if wiki, err = confluence.NewWiki(rootCmd.baseUrl, confluence.BasicAuth(rootCmd.userName, rootCmd.password)); err != nil {
		return err
	}

	if page, err = wiki.GetContent(pageId, []string{"body", "version"}); err != nil {
		return err
	}

	// TODO: update Title

	page.Body.Storage.Value = string(content)
	page.Body.Storage.Representation = "wiki"
	page.Version.Number += 1
	if _, err := wiki.UpdateContent(page); err != nil {
		return err
	}

	// TODO: upload attachments
	// curl -v -S -u admin:admin -X POST -H "X-Atlassian-Token: no-check" -F "file=@myfile.txt" -F
	//"comment=this is my file" "http://localhost:8080/confluence/rest/api/content/3604482/child/attachment"

	return nil
}
