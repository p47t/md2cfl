package bf2confluence

import (
	"fmt"
	"io"

	bf "github.com/russross/blackfriday/v2"
)

// Renderer is the rendering interface for bf2confluence wiki output.
type XmlRenderer struct {
	Renderer
	inTableHeader bool
}

func (r *XmlRenderer) cdata(w io.Writer, content []byte) {
	w.Write([]byte("<![CDATA["))
	w.Write(content)
	w.Write([]byte("]]>"))
}

func (r *XmlRenderer) tag(w io.Writer, t []byte, content []byte) {
	r.openTag(w, t)
	w.Write(content)
	r.closeTag(w, t)
}

func (r *XmlRenderer) openTag(w io.Writer, t []byte) {
	w.Write([]byte("<"))
	w.Write(t)
	r.out(w, []byte(">"))
}

func (r *XmlRenderer) closeTag(w io.Writer, t []byte) {
	w.Write([]byte("</"))
	w.Write(t)
	r.out(w, []byte(">"))
}

func (r *XmlRenderer) openStructuredMacro(w io.Writer, name string) {
	r.openTag(w, []byte(fmt.Sprintf(`ac:structured-macro ac:name="%s"`, name)))
}

func (r *XmlRenderer) openParameter(w io.Writer, name string) {
	r.openTag(w, []byte(fmt.Sprintf(`ac:parameter ac:name="%s"`, name)))
}

func (r *XmlRenderer) closeParameter(w io.Writer) {
	r.closeTag(w, []byte(`ac:parameter`))
}

var example = `<ac:structured-macro ac:name="code" ac:schema-version="1" ac:macro-id="3f81cd1d-5386-43ce-bd9b-aa8619f43244">
<ac:parameter ac:name="">bash</ac:parameter><ac:plain-text-body><![CDATA[
	$ ulimit -c unlimited
	]]></ac:plain-text-body></ac:structured-macro>`

func (r *XmlRenderer) closeStructuredMacro(w io.Writer) {
	r.closeTag(w, []byte(`ac:structured-macro`))
}

// RenderNode is a bf2confluence renderer of a single node of a syntax tree.
func (r *XmlRenderer) RenderNode(w io.Writer, node *bf.Node, entering bool) bf.WalkStatus {
	switch node.Type {
	case bf.Text:
		r.out(w, node.Literal)
	case bf.Softbreak:
		break
	case bf.Hardbreak:
		break
	case bf.BlockQuote:
		if entering {
			r.openTag(w, []byte("blockquote"))
		} else {
			r.closeTag(w, []byte("blockquote"))
		}
	case bf.CodeBlock:
		if len(node.Info) > 0 {
			language := string(node.Info)
			r.openStructuredMacro(w, "code")
			r.openParameter(w, "")
			r.out(w, []byte(language))
			r.closeParameter(w)
			r.openTag(w, []byte("ac:plain-text-body"))
			r.cdata(w, node.Literal)
			r.closeTag(w, []byte("ac:plain-text-body"))
			r.closeStructuredMacro(w)
			r.cr(w)
		}
	case bf.Code:
		if entering {
			r.openTag(w, []byte("code"))
		} else {
			r.closeTag(w, []byte("code"))
		}
	case bf.Emph:
		if entering {
			r.openTag(w, []byte("em"))
		} else {
			r.closeTag(w, []byte("em"))
		}
	case bf.Heading:
		headingTag := []byte(fmt.Sprintf("h%d", node.Level))
		if entering {
			r.openTag(w, headingTag)
		} else {
			r.closeTag(w, headingTag)
			r.cr(w)
		}
	case bf.Image:
		if entering {
			r.openTag(w, []byte(`ac:image href="%s"`))
			r.openTag(w, []byte(fmt.Sprintf(`ri:url ri:value="%s"`, node.LinkData.Destination)))
			r.closeTag(w, []byte("ri:url"))
		} else {
			r.closeTag(w, []byte("ac:image"))
		}
	case bf.Item:
		if entering {
			r.openTag(w, []byte("li"))
		} else {
			r.closeTag(w, []byte("li"))
		}
	case bf.Link:
		if entering {
			r.openTag(w, []byte(fmt.Sprintf(`a href="%s"`, node.LinkData.Destination)))
		} else {
			r.closeTag(w, []byte("a"))
		}
	case bf.HorizontalRule:
		r.out(w, []byte(`<hr />`))
		r.cr(w)
	case bf.List:
		if entering {
			r.openTag(w, []byte("ul"))
		} else {
			r.closeTag(w, []byte("ul"))
		}
	case bf.Document:
		break
	case bf.Paragraph:
		if entering {
			r.openTag(w, []byte("p"))
		} else {
			r.closeTag(w, []byte("p"))
			r.cr(w)
		}
	case bf.Strong:
		if entering {
			r.openTag(w, []byte("strong"))
		} else {
			r.closeTag(w, []byte("strong"))
		}
	case bf.Del:
		if entering {
			r.openTag(w, []byte(`span style="text-decoration: line-through;"`))
		} else {
			r.closeTag(w, []byte("span"))
		}
	case bf.Table:
		if entering {
			r.openTag(w, []byte("table"))
			r.openTag(w, []byte("tbody"))
		} else {
			r.closeTag(w, []byte("tbody"))
			r.closeTag(w, []byte("table"))
		}
	case bf.TableCell:
		if entering {
			if r.inTableHeader {
				r.openTag(w, []byte("th"))
			} else {
				r.openTag(w, []byte("td"))
			}
		} else {
			if r.inTableHeader {
				r.closeTag(w, []byte("th"))
			} else {
				r.closeTag(w, []byte("td"))
			}
		}
	case bf.TableHead:
		if entering {
			r.inTableHeader = true
		} else {
			r.inTableHeader = false
		}
	case bf.TableBody:
	case bf.TableRow:
		if entering {
			r.openTag(w, []byte("tr"))
		} else {
			r.closeTag(w, []byte("tr"))
		}
	case bf.HTMLBlock:
		if entering {
			r.openTag(w, []byte("div"))
		} else {
			r.closeTag(w, []byte("div"))
		}
	case bf.HTMLSpan:
		r.out(w, node.Literal)
	case bf.Macro:
		r.openStructuredMacro(w, node.Name)
		for param, value := range node.Parameters {
			r.openParameter(w, param)
			r.cdata(w, []byte(value))
			r.closeParameter(w)
		}
		r.closeStructuredMacro(w)
	default:
		panic("Unknown node type " + node.Type.String())
	}
	return bf.GoToNext
}

// Render prints out the whole document from the ast.
func (r *XmlRenderer) Render(ast *bf.Node) []byte {
	ast.Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
		return r.RenderNode(&r.w, node, entering)
	})

	return r.w.Bytes()
}
