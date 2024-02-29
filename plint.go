package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/gammazero/deque"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
)

var (
	cLang = c.GetLanguage()

	lipRedBold = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true).Render
	lipBold    = lipgloss.NewStyle().Bold(true).Render
	lipBlue    = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Render
	lipGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render
)

func main() {
	err := Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func Run(args []string) error {
	switch len(args) {
	case 2:
		hdrFname := args[1]
		hdr, err := os.ReadFile(hdrFname)
		if err != nil {
			return err
		}
		err = Validate(hdrFname, hdr, hdrFname, hdr)
		if err != nil {
			return err
		}
	case 3:
		hdrFname := args[1]
		srcFname := args[2]
		hdr, err := os.ReadFile(hdrFname)
		if err != nil {
			return err
		}
		src, err := os.ReadFile(srcFname)
		if err != nil {
			return err
		}
		err = Validate(hdrFname, hdr, srcFname, src)
		if err != nil {
			return err
		}
	default:
		return makeError(fmt.Sprintf("Usage: %s file [file]", args[0]))
	}
	return nil
}

func Validate(hdrFname string, hdr []byte, srcFname string, src []byte) error {
	p := sitter.NewParser()
	p.SetLanguage(cLang)

	fnIdNodes := deque.New[*sitter.Node]()

	t, err := p.ParseCtx(context.Background(), nil, hdr)
	if err != nil {
		return err
	}
	r := t.RootNode()
	err = assertNoParserErrors(r, hdrFname, hdr)
	if err != nil {
		return err
	}
	qc := tsQueryCursor(r, "[(declaration) @decl (function_definition) @def]")

	var m *sitter.QueryMatch
	var present bool

outer:
	for m, present = qc.NextMatch(); present; m, present = qc.NextMatch() {
		for _, c := range m.Captures {
			if c.Node.Type() == "declaration" {
				idNode, present := getFnIdNode(c.Node)
				if present {
					fnIdNodes.PushBack(idNode)
				}
			} else if c.Node.Type() == "function_definition" {
				break outer
			} else {
				log.Panicln("unexpected node type:", c.Node.Type())
			}
		}
	}

	if hdrFname == srcFname && present {
		err := validateFns(m, hdrFname, hdr, srcFname, src, fnIdNodes)
		if err != nil {
			return err
		}
		for m, present := qc.NextMatch(); present; m, present = qc.NextMatch() {
			err := validateFns(m, hdrFname, hdr, srcFname, src, fnIdNodes)
			if err != nil {
				return err
			}
		}
	} else if hdrFname == srcFname {
		if fnIdNodes.Len() > 0 {
			return makeError(
				renderErrorString("declaration without matching definition"),
				renderNodeLocString(fnIdNodes.PopFront(), hdrFname, hdr),
			)
		}
		return makeError(
			renderErrorString("header-only file doesn't contain any definition"),
		)
	} else {
		t, err = p.ParseCtx(context.Background(), nil, src)
		if err != nil {
			return err
		}
		r := t.RootNode()
		err = assertNoParserErrors(r, srcFname, src)
		if err != nil {
			return err
		}
		qc = tsQueryCursor(r, "[(declaration) @decl (function_definition) @def]")

		for m, present := qc.NextMatch(); present; m, present = qc.NextMatch() {
			err := validateFns(m, hdrFname, hdr, srcFname, src, fnIdNodes)
			if err != nil {
				return err
			}
		}
	}

	if fnIdNodes.Len() > 0 {
		return makeError(
			renderErrorString("declaration without matching definition"),
			renderNodeLocString(fnIdNodes.PopFront(), hdrFname, hdr),
		)
	}
	return nil
}

func validateFns(m *sitter.QueryMatch,
	hdrFname string, hdrData []byte,
	srcFname string, srcData []byte,
	hdrFnIdNodes *deque.Deque[*sitter.Node]) error {
	for _, c := range m.Captures {
		n := c.Node
		if n.Type() == "function_definition" {
			if isStaticFn(n, srcData) {
				continue
			}

			srcFnIdNode, present := getFnIdNode(n)
			if !present {
				continue
			}

			srcFnName := srcFnIdNode.Content(srcData)
			if srcFnName == "main" {
				continue
			}

			if hdrFnIdNodes.Len() == 0 {
				return makeError(
					renderErrorString("definition without matching declaration"),
					renderNodeLocString(srcFnIdNode, hdrFname, hdrData),
				)
			}

			hdrFnIdNode := hdrFnIdNodes.PopFront()
			hdrFnName := hdrFnIdNode.Content(hdrData)
			if srcFnName == hdrFnName {
				continue
			}

			return makeError(
				renderErrorString("declaration and definition not in order"),
				renderNodeLocString(hdrFnIdNode, hdrFname, hdrData),
				"",
				renderNodeLocString(srcFnIdNode, srcFname, srcData),
			)
		} else if n.Type() == "declaration" {
			_, present := getFnIdNode(n)
			if !present {
				continue
			}

			return makeError(
				renderErrorString("found declaration but expected definition"),
				renderNodeLocString(n, srcFname, srcData),
			)
		} else {
			log.Panicln("unexpected node type:", n.Type())
		}
	}
	return nil
}

func assertNoParserErrors(n *sitter.Node, fname string, fdata []byte) error {
	if n.IsError() {
		return makeError(
			renderErrorString("syntax error"),
			renderNodeLocString(n, fname, fdata),
		)
	}
	for i := 0; i < int(n.ChildCount()); i += 1 {
		err := assertNoParserErrors(n.Child(i), fname, fdata)
		if err != nil {
			return err
		}
	}
	return nil
}

func tsQueryCursor(n *sitter.Node, query string) *sitter.QueryCursor {
	q, err := sitter.NewQuery([]byte(query), cLang)
	if err != nil {
		log.Panicln(err)
	}
	c := sitter.NewQueryCursor()
	c.Exec(q, n)
	return c
}

func getFnIdNode(n *sitter.Node) (*sitter.Node, bool) {
	c := tsQueryCursor(n, "(function_declarator declarator: (identifier) @id)")
	m, present := c.NextMatch()
	if !present {
		return nil, false
	}
	return m.Captures[0].Node, true
}

func makeError(lines ...any) error {
	b := strings.Builder{}
	for _, l := range lines {
		b.WriteString(fmt.Sprintln(l))
	}
	return errors.New(b.String())
}

func renderErrorString(msg string) string {
	b := strings.Builder{}
	b.WriteString(lipRedBold("error:"))
	b.WriteRune(' ')
	b.WriteString(lipBold(msg))
	return b.String()
}

func renderNodeLocString(n *sitter.Node, fname string, fdata []byte) string {
	row := int(n.StartPoint().Row)
	col := int(n.StartPoint().Column)
	rowNumWidth := intToStringWidth(row)

	b := strings.Builder{}
	b.WriteString(lipBlue(fmt.Sprintf("%s:%d:%d", fname, row+1, col+1)))
	b.WriteString(lipGreen(fmt.Sprintf("\n%-*s |", rowNumWidth, "")))
	b.WriteString(lipGreen(fmt.Sprintf("\n%-*d |", rowNumWidth, row+1)))
	b.WriteString(fmt.Sprintf(" %s", stringAtRow(fdata, row)))
	b.WriteString(lipGreen(fmt.Sprintf("%-*s |", rowNumWidth, "")))
	return b.String()
}

func isStaticFn(n *sitter.Node, buf []byte) bool {
	found := false
	for i := 0; i < int(n.ChildCount()); i += 1 {
		c := n.Child(i)
		if c.Type() == "storage_class_specifier" && c.Content(buf) == "static" {
			found = true
			break
		}
	}
	return found
}

func intToStringWidth(n int) int {
	width := 1
	if n < 0 {
		n *= -1
		width += 1
	}
	for n >= 10 {
		width += 1
		n /= 10
	}
	return width
}

func stringAtRow(fdata []byte, frow int) string {
	r := 0
	beg := 0
	for ; r < frow; beg += 1 {
		if fdata[beg] == '\n' {
			r += 1
		}
	}
	end := beg
	for ; fdata[end] != '\n'; end += 1 {
	}
	end += 1
	return string(fdata[beg:end])
}
