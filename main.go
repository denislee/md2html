package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type model struct {
	files   []string
	cursor  int
	chosen  string
	err     error
	done    bool
	outPath string
}

func initialModel() (model, error) {
	files, err := findMarkdownFiles(".")
	if err != nil {
		return model{}, err
	}
	if len(files) == 0 {
		return model{}, fmt.Errorf("no markdown files found in current directory")
	}
	return model{files: files}, nil
}

func findMarkdownFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".md" || ext == ".markdown" {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files, nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.chosen = m.files[m.cursor]
			out, err := convertAndOpen(m.chosen)
			if err != nil {
				m.err = err
			} else {
				m.outPath = out
				m.done = true
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.done {
		return selectedStyle.Render(fmt.Sprintf("Opened %s\nHTML: %s\n", m.chosen, m.outPath))
	}
	if m.err != nil {
		return errStyle.Render(fmt.Sprintf("Error: %v\n", m.err))
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Select a markdown file to convert") + "\n\n")
	for i, f := range m.files {
		if i == m.cursor {
			b.WriteString(cursorStyle.Render("> "+f) + "\n")
		} else {
			b.WriteString("  " + f + "\n")
		}
	}
	b.WriteString("\n" + helpStyle.Render("↑/↓ or j/k to move • enter to select • q to quit"))
	return b.String()
}

func convertMarkdown(src []byte) ([]byte, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Table, extension.Strikethrough, extension.TaskList),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	var buf strings.Builder
	if err := md.Convert(src, &buf); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>%s</title>
<style>
body { max-width: 800px; margin: 2rem auto; padding: 0 1rem; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; line-height: 1.6; color: #24292f; }
pre { background: #f6f8fa; padding: 1rem; border-radius: 6px; overflow-x: auto; }
code { background: #f6f8fa; padding: 0.2em 0.4em; border-radius: 3px; font-size: 85%%; }
pre code { background: transparent; padding: 0; }
blockquote { border-left: 4px solid #d0d7de; padding-left: 1rem; color: #57606a; margin-left: 0; }
table { border-collapse: collapse; }
th, td { border: 1px solid #d0d7de; padding: 0.4rem 0.8rem; }
img { max-width: 100%%; }
a { color: #0969da; }
</style>
</head>
<body>
%s
</body>
</html>`

func convertAndOpen(path string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	body, err := convertMarkdown(src)
	if err != nil {
		return "", err
	}
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	full := fmt.Sprintf(htmlTemplate, title, string(body))

	tmp, err := os.CreateTemp("", "md2html-*.html")
	if err != nil {
		return "", err
	}
	if _, err := tmp.WriteString(full); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := openBrowser(tmp.Name()); err != nil {
		return tmp.Name(), err
	}
	return tmp.Name(), nil
}

func openBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func main() {
	m, err := initialModel()
	if err != nil {
		fmt.Fprintln(os.Stderr, errStyle.Render(err.Error()))
		os.Exit(1)
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
