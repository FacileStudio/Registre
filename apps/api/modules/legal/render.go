package legal

import (
	"html"
	"regexp"
	"strings"
)

// Render turns the subset of markdown the legal texts use into HTML and
// answers the first-level title separately: "# " title, "## " and "### "
// headings, "- " list items, blank-line paragraphs, **bold** and
// [label](https://… or /…) links. Every character is escaped before any tag
// is added, so a text can never inject markup.
func Render(source string) (title, body string) {
	var r renderer
	for _, raw := range strings.Split(source, "\n") {
		r.line(strings.TrimSpace(raw))
	}
	r.flush()
	return r.title, r.out.String()
}

// renderer is the state of one Render call.
type renderer struct {
	out       strings.Builder
	title     string
	paragraph []string
	inList    bool
}

// line consumes one source line.
func (r *renderer) line(line string) {
	switch {
	case strings.HasPrefix(line, "# ") && r.title == "":
		r.title = trimTitle(line)
	case line == "":
		r.flush()
	case strings.HasPrefix(line, "#"):
		r.flush()
		r.out.WriteString(heading(line))
	case strings.HasPrefix(line, "- "):
		r.item(strings.TrimPrefix(line, "- "))
	default:
		r.paragraph = append(r.paragraph, line)
	}
}

// item writes one list item, opening the list when needed.
func (r *renderer) item(text string) {
	if len(r.paragraph) > 0 {
		r.flush()
	}
	if !r.inList {
		r.out.WriteString("<ul>\n")
		r.inList = true
	}
	r.out.WriteString("<li>" + inline(text) + "</li>\n")
}

// flush closes the open paragraph or list.
func (r *renderer) flush() {
	if len(r.paragraph) > 0 {
		r.out.WriteString("<p>" + inline(strings.Join(r.paragraph, " ")) + "</p>\n")
		r.paragraph = nil
	}
	if r.inList {
		r.out.WriteString("</ul>\n")
		r.inList = false
	}
}

// heading renders "## " as h2 and anything deeper as h3.
func heading(line string) string {
	if strings.HasPrefix(line, "## ") {
		return "<h2>" + inline(strings.TrimPrefix(line, "## ")) + "</h2>\n"
	}
	return "<h3>" + inline(strings.TrimLeft(line, "# ")) + "</h3>\n"
}

// inline escapes text, then adds bold and links.
func inline(text string) string {
	escaped := html.EscapeString(text)
	escaped = boldPattern().ReplaceAllString(escaped, "<strong>$1</strong>")
	return linkPattern().ReplaceAllString(escaped, `<a href="$2">$1</a>`)
}

// boldPattern matches **text**.
func boldPattern() *regexp.Regexp { return regexp.MustCompile(`\*\*([^*]+)\*\*`) }

// linkPattern matches [label](target) with an https or site-relative target.
func linkPattern() *regexp.Regexp {
	return regexp.MustCompile(`\[([^\]]+)\]\(((?:https://|/)[^)\s"]*)\)`)
}
