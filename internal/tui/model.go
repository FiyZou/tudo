package tui

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FiyZou/tudo/internal/config"
	"github.com/FiyZou/tudo/internal/storage"
)

type model struct {
	store *storage.Store
	cfg   config.Config
	input textinput.Model
	todos []storage.Todo

	cursor int
	page   int
	width  int
	height int

	styles  styles
	errText string
	help    bool

	commandCursor int
}

type commandDef struct {
	Name        string
	Usage       string
	Description string
}

var commands = []commandDef{
	{Name: "/add", Usage: "/add <text>", Description: "add a todo"},
	{Name: "/edit", Usage: "/edit <id> <text>", Description: "update a todo"},
	{Name: "/delete", Usage: "/delete <id>", Description: "delete a todo"},
	{Name: "/done", Usage: "/done <id>", Description: "mark a todo completed"},
	{Name: "/undo", Usage: "/undo <id>", Description: "reopen a completed todo"},
	{Name: "/help", Usage: "/help", Description: "toggle command help"},
	{Name: "/exit", Usage: "/exit", Description: "exit Tudo"},
}

func New(store *storage.Store, cfg config.Config) tea.Model {
	s := newStyles(cfg.UI, cfg.Colors)

	input := textinput.New()
	input.Placeholder = ""
	input.Focus()
	input.Prompt = cfg.UI.Prompt
	input.CharLimit = 500
	inputTextStyle := lipgloss.NewStyle().
		Foreground(s.inputForeground).
		Background(s.inputBackground)
	input.PromptStyle = inputTextStyle
	input.TextStyle = inputTextStyle
	input.Cursor.Style = lipgloss.NewStyle()
	input.PlaceholderStyle = inputTextStyle
	input.SetCursorMode(textinput.CursorHide)

	m := model{
		store:  store,
		cfg:    cfg,
		input:  input,
		styles: s,
	}
	m.reload()
	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		inputPaddingX := max(0, m.cfg.UI.InputPaddingX)
		m.input.Width = max(1, msg.Width-(inputPaddingX*2))
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.commandPaletteOpen() {
				m.moveCommandCursor(-1)
				return m, nil
			}
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down":
			if m.commandPaletteOpen() {
				m.moveCommandCursor(1)
				return m, nil
			}
			if m.cursor < m.currentPageSize()-1 {
				m.cursor++
			}
			return m, nil
		case "j":
			if !m.commandPaletteOpen() && m.input.Value() == "" {
				m.previousPage()
				return m, nil
			}
		case "k":
			if !m.commandPaletteOpen() && m.input.Value() == "" {
				m.nextPage()
				return m, nil
			}
		case "tab":
			if m.commandPaletteOpen() {
				m.completeSelectedCommand()
				return m, nil
			}
		case "ctrl+j", "cmd+j", "super+j", "meta+j":
			if m.commandPaletteOpen() {
				return m, nil
			}
			m.toggleSelectedDone()
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				return m, nil
			}
			if m.commandPaletteOpen() && m.shouldCompleteCommand(value) {
				m.completeSelectedCommand()
				return m, nil
			}
			m.input.SetValue("")
			if value == "/exit" {
				return m, tea.Quit
			}
			m.runCommand(value)
			return m, nil
		}
	}

	m.input, cmd = m.input.Update(msg)
	m.clampCommandCursor()
	return m, cmd
}

func (m *model) runCommand(value string) {
	m.errText = ""
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return
	}
	if !strings.HasPrefix(parts[0], "/") {
		m.apply(func() error { return m.store.AddTodo(value) })
		return
	}

	command := strings.ToLower(parts[0])
	switch command {
	case "/help":
		m.help = !m.help
	case "/add":
		title := strings.TrimSpace(strings.TrimPrefix(value, parts[0]))
		if title == "" {
			m.errText = "/add needs text"
			return
		}
		m.apply(func() error { return m.store.AddTodo(title) })
	case "/edit":
		id, rest, err := parseIDAndRest(parts, value)
		if err != nil {
			m.errText = "usage: /edit <id> <new text>"
			return
		}
		if strings.TrimSpace(rest) == "" {
			m.errText = "/edit needs new text"
			return
		}
		m.apply(func() error { return m.store.UpdateTodo(id, rest) })
	case "/delete":
		id, err := parseID(parts)
		if err != nil {
			m.errText = "usage: /delete <id>"
			return
		}
		m.apply(func() error { return m.store.DeleteTodo(id) })
	case "/done":
		id, err := parseID(parts)
		if err != nil {
			m.errText = "usage: /done <id>"
			return
		}
		m.apply(func() error { return m.store.SetCompleted(id, true) })
	case "/undo":
		id, err := parseID(parts)
		if err != nil {
			m.errText = "usage: /undo <id>"
			return
		}
		m.apply(func() error { return m.store.SetCompleted(id, false) })
	default:
		m.errText = fmt.Sprintf("unknown command %s", parts[0])
	}
}

func (m *model) apply(action func() error) {
	if err := action(); err != nil {
		m.errText = err.Error()
		return
	}
	m.reload()
}

func (m *model) reload() {
	todos, err := m.store.ListTodos()
	if err != nil {
		m.errText = err.Error()
		return
	}
	m.todos = todos
	m.clampPage()
}

func (m model) View() string {
	width := m.width
	if width == 0 {
		width = 88
	}
	if m.height == 1 {
		return m.renderHeader(width)
	}

	bottom := m.renderBottom(width)
	bottomHeight := lipgloss.Height(bottom)
	maxContentHeight := 0
	if m.height > 0 {
		maxContentHeight = max(0, m.height-1-bottomHeight)
	}

	contentLines := []string{
		m.renderStats(width),
		"",
	}
	if m.help && m.cfg.UI.ShowCommands {
		contentLines = append(contentLines, m.styles.help.Width(width).Render("Commands: "+commandUsageText()), "")
	}

	if len(m.todos) == 0 {
		contentLines = append(contentLines, m.styles.empty.Width(width).Render("No todos yet. Type one below and press Enter."))
	} else {
		contentLines = append(contentLines, strings.Split(m.renderTodos(width, max(1, maxContentHeight-len(contentLines))), "\n")...)
	}

	if m.errText != "" {
		contentLines = append(contentLines, "", m.styles.err.Width(width).Render(m.errText))
	}

	if m.height > 0 {
		if maxContentHeight == 0 {
			contentLines = nil
		} else if len(contentLines) > maxContentHeight {
			contentLines = contentLines[:maxContentHeight]
		}
	}

	parts := []string{m.renderHeader(width)}
	if len(contentLines) > 0 {
		parts = append(parts, strings.Join(contentLines, "\n"))
	}
	content := strings.Join(parts, "\n")
	if m.height > 0 {
		padding := m.height - lipgloss.Height(content) - bottomHeight
		if padding > 0 {
			content += strings.Repeat("\n", padding)
		}
	}
	content += "\n" + bottom

	return content
}

func (m model) renderHeader(width int) string {
	title := " Tudo"
	padding := max(0, width-lipgloss.Width(title))
	return m.styles.header.Render(title + strings.Repeat(" ", padding))
}

func (m model) matchingCommands() []commandDef {
	value := m.input.Value()
	if !strings.HasPrefix(value, "/") {
		return nil
	}
	if strings.ContainsAny(value, " \t\n") {
		return nil
	}

	query := strings.ToLower(value)
	matches := make([]commandDef, 0, len(commands))
	for _, command := range commands {
		if strings.HasPrefix(command.Name, query) {
			matches = append(matches, command)
		}
	}
	return matches
}

func (m model) renderCommandSuggestions(width int, suggestions []commandDef, maxLines int) string {
	lines := make([]string, 0, maxLines)
	for i, suggestion := range suggestions {
		if len(lines) >= maxLines {
			break
		}
		prefix := "  "
		style := m.styles.suggestion
		if i == m.selectedCommandIndex(len(suggestions)) {
			prefix = "› "
			style = m.styles.suggestionSelected
		}

		line := fmt.Sprintf("%s%-18s %s", prefix, suggestion.Usage, suggestion.Description)
		lines = append(lines, style.Width(width).Render(line))
	}
	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m model) renderBottom(width int) string {
	maxSuggestionLines := len(commands)
	if m.height > 0 {
		inputHeight := lipgloss.Height(m.renderInput(width))
		maxSuggestionLines = max(0, m.height-1-inputHeight)
		maxSuggestionLines = min(maxSuggestionLines, len(commands))
	}

	var lines []string
	if suggestions := m.matchingCommands(); len(suggestions) > 0 && maxSuggestionLines > 0 {
		lines = append(lines, m.renderCommandSuggestions(width, suggestions, maxSuggestionLines))
	}
	lines = append(lines, m.renderInput(width))
	return strings.Join(lines, "\n")
}

func (m model) commandPaletteOpen() bool {
	return len(m.matchingCommands()) > 0
}

func (m *model) moveCommandCursor(delta int) {
	count := len(m.matchingCommands())
	if count == 0 {
		m.commandCursor = 0
		return
	}
	m.commandCursor = (m.commandCursor + delta + count) % count
}

func (m *model) clampCommandCursor() {
	count := len(m.matchingCommands())
	if count == 0 {
		m.commandCursor = 0
		return
	}
	if m.commandCursor >= count {
		m.commandCursor = count - 1
	}
	if m.commandCursor < 0 {
		m.commandCursor = 0
	}
}

func (m model) selectedCommandIndex(count int) int {
	if count <= 0 {
		return 0
	}
	if m.commandCursor >= count {
		return count - 1
	}
	if m.commandCursor < 0 {
		return 0
	}
	return m.commandCursor
}

func (m model) shouldCompleteCommand(value string) bool {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return false
	}
	if len(parts) > 1 {
		return false
	}

	selected, ok := m.selectedCommand()
	return ok && parts[0] != selected.Name
}

func (m *model) completeSelectedCommand() {
	selected, ok := m.selectedCommand()
	if !ok {
		return
	}

	m.input.SetValue(selected.Name + " ")
	m.input.CursorEnd()
	m.commandCursor = 0
}

func (m model) selectedCommand() (commandDef, bool) {
	matches := m.matchingCommands()
	if len(matches) == 0 {
		return commandDef{}, false
	}
	return matches[m.selectedCommandIndex(len(matches))], true
}

func (m model) renderInput(width int) string {
	paddingY := max(0, m.cfg.UI.InputPaddingY)
	paddingX := max(0, m.cfg.UI.InputPaddingX)
	if m.height > 0 {
		maxInputHeight := max(1, m.height-1)
		paddingY = min(paddingY, max(0, (maxInputHeight-1)/2))
	}

	blankLine := m.styles.inputFrame.Render(strings.Repeat(" ", width))
	lines := make([]string, 0, paddingY*2+1)
	for range paddingY {
		lines = append(lines, blankLine)
	}

	input := m.renderInputText()
	inputWidth := lipgloss.Width(input)
	rightPadding := max(0, width-paddingX-inputWidth)
	line := m.styles.inputFrame.Render(strings.Repeat(" ", paddingX)) +
		m.styles.inputText.Render(input) +
		m.styles.inputFrame.Render(strings.Repeat(" ", rightPadding))
	lines = append(lines, line)

	for range paddingY {
		lines = append(lines, blankLine)
	}

	return strings.Join(lines, "\n")
}

func (m model) renderInputText() string {
	value := []rune(m.input.Value())
	position := m.input.Position()
	if position < 0 {
		position = 0
	}
	if position > len(value) {
		position = len(value)
	}

	before := string(value[:position])
	after := string(value[position:])

	return m.styles.inputText.Render(m.cfg.UI.Prompt+before) +
		m.styles.inputCursor.Render("▏") +
		m.styles.inputText.Render(after)
}

func (m model) renderStats(width int) string {
	openCount := 0
	for _, t := range m.todos {
		if !t.Completed {
			openCount++
		}
	}

	text := fmt.Sprintf("%d open, %d total", openCount, len(m.todos))
	if len(m.todos) > 0 {
		text += fmt.Sprintf("  |  page %d/%d  |  j prev  k next", m.page+1, m.totalPages())
	}
	if m.cfg.UI.ShowPaths {
		text += fmt.Sprintf("  |  data %s  |  config %s", m.cfg.DatabasePath, m.cfg.ConfigPath)
	}

	return m.styles.meta.Width(width).Render(text)
}

func (m model) renderTodos(width int, maxContentRows int) string {
	maxRows := m.pageSize()
	if maxContentRows > 0 {
		maxRows = min(maxRows, maxContentRows)
	}
	if maxRows < 1 {
		maxRows = 1
	}

	start := min(len(m.todos), m.page*m.pageSize())
	end := min(len(m.todos), start+maxRows)

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		t := m.todos[i]
		pointer := " "
		if i-start == m.cursor {
			pointer = ">"
		}

		box := "[ ]"
		style := m.styles.item
		if t.Completed {
			box = "[x]"
			style = m.styles.done
		}

		line := fmt.Sprintf("%s %s #%d %s", pointer, box, t.ID, t.Title)
		lines = append(lines, style.Width(width).Render(line))
	}

	return strings.Join(lines, "\n")
}

func (m model) pageSize() int {
	if m.cfg.UI.PageSize <= 0 {
		return 12
	}
	return m.cfg.UI.PageSize
}

func (m model) totalPages() int {
	if len(m.todos) == 0 {
		return 1
	}
	return (len(m.todos) + m.pageSize() - 1) / m.pageSize()
}

func (m model) currentPageSize() int {
	if len(m.todos) == 0 {
		return 0
	}
	start := m.page * m.pageSize()
	if start >= len(m.todos) {
		return 0
	}
	return min(m.pageSize(), len(m.todos)-start)
}

func (m model) selectedTodo() (storage.Todo, bool) {
	index := m.page*m.pageSize() + m.cursor
	if index < 0 || index >= len(m.todos) {
		return storage.Todo{}, false
	}
	return m.todos[index], true
}

func (m *model) toggleSelectedDone() {
	todo, ok := m.selectedTodo()
	if !ok {
		return
	}
	m.apply(func() error { return m.store.SetCompleted(todo.ID, !todo.Completed) })
}

func (m *model) nextPage() {
	if m.page < m.totalPages()-1 {
		m.page++
		m.cursor = 0
	}
}

func (m *model) previousPage() {
	if m.page > 0 {
		m.page--
		m.cursor = 0
	}
}

func (m *model) clampPage() {
	if m.page >= m.totalPages() {
		m.page = max(0, m.totalPages()-1)
	}
	if m.page < 0 {
		m.page = 0
	}
	if m.cursor >= m.currentPageSize() {
		m.cursor = max(0, m.currentPageSize()-1)
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func parseID(parts []string) (int64, error) {
	if len(parts) < 2 {
		return 0, errors.New("missing id")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func parseIDAndRest(parts []string, full string) (int64, string, error) {
	id, err := parseID(parts)
	if err != nil {
		return 0, "", err
	}

	prefix := parts[0] + " " + parts[1]
	rest := strings.TrimSpace(strings.TrimPrefix(full, prefix))
	return id, rest, nil
}

type styles struct {
	header             lipgloss.Style
	meta               lipgloss.Style
	err                lipgloss.Style
	help               lipgloss.Style
	suggestion         lipgloss.Style
	suggestionSelected lipgloss.Style
	empty              lipgloss.Style
	item               lipgloss.Style
	done               lipgloss.Style
	inputFrame         lipgloss.Style
	inputText          lipgloss.Style
	inputCursor        lipgloss.Style
	inputBackground    lipgloss.Color
	inputForeground    lipgloss.Color
}

func newStyles(ui config.UIConfig, colors config.ColorsConfig) styles {
	inputBackground := lipgloss.Color(colors.InputBackground)
	inputForeground := contrastColor(inputBackground)

	return styles{
		header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colors.HeaderForeground)).
			Background(lipgloss.Color(colors.HeaderBackground)).
			Padding(0, 1),
		meta: lipgloss.NewStyle().
			Foreground(adaptive(colors.MetaForegroundLight, colors.MetaForegroundDark)),
		err: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.ErrorForeground)),
		help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.HelpForeground)).
			Background(lipgloss.Color(colors.HelpBackground)).
			Padding(0, 1),
		suggestion: lipgloss.NewStyle().
			Foreground(adaptive(colors.MetaForegroundLight, colors.MetaForegroundDark)),
		suggestionSelected: lipgloss.NewStyle().
			Foreground(inputForeground).
			Background(inputBackground),
		empty: lipgloss.NewStyle().
			Foreground(adaptive(colors.EmptyForegroundLight, colors.EmptyForegroundDark)).
			Italic(true),
		item: lipgloss.NewStyle().
			Foreground(adaptive(colors.ListForegroundLight, colors.ListForegroundDark)),
		done: lipgloss.NewStyle().
			Foreground(adaptive(colors.DoneForegroundLight, colors.DoneForegroundDark)).
			Strikethrough(true),
		inputFrame: lipgloss.NewStyle().
			Background(inputBackground),
		inputText: lipgloss.NewStyle().
			Foreground(inputForeground).
			Background(inputBackground),
		inputCursor: lipgloss.NewStyle().
			Foreground(inputForeground).
			Background(inputBackground),
		inputBackground: inputBackground,
		inputForeground: inputForeground,
	}
}

func adaptive(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

func commandUsageText() string {
	usages := make([]string, 0, len(commands))
	for _, command := range commands {
		usages = append(usages, command.Usage)
	}
	return strings.Join(usages, " | ")
}

func contrastColor(background lipgloss.Color) lipgloss.Color {
	if colorLuminance(string(background)) > 0.5 {
		return lipgloss.Color("232")
	}
	return lipgloss.Color("244")
}

func colorLuminance(value string) float64 {
	if index, err := strconv.Atoi(value); err == nil {
		r, g, b := ansi256ToRGB(index)
		return relativeLuminance(r, g, b)
	}

	hex := strings.TrimPrefix(value, "#")
	if len(hex) == 6 {
		r, errR := strconv.ParseInt(hex[0:2], 16, 64)
		g, errG := strconv.ParseInt(hex[2:4], 16, 64)
		b, errB := strconv.ParseInt(hex[4:6], 16, 64)
		if errR == nil && errG == nil && errB == nil {
			return relativeLuminance(int(r), int(g), int(b))
		}
	}

	return 0
}

func ansi256ToRGB(index int) (int, int, int) {
	if index < 0 {
		index = 0
	}
	if index > 255 {
		index = 255
	}

	if index < 16 {
		base := [16][3]int{
			{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
			{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
			{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
			{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
		}
		color := base[index]
		return color[0], color[1], color[2]
	}

	if index >= 232 {
		level := 8 + (index-232)*10
		return level, level, level
	}

	index -= 16
	steps := [6]int{0, 95, 135, 175, 215, 255}
	return steps[index/36], steps[(index/6)%6], steps[index%6]
}

func relativeLuminance(red, green, blue int) float64 {
	r := linearRGB(float64(red) / 255)
	g := linearRGB(float64(green) / 255)
	b := linearRGB(float64(blue) / 255)
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func linearRGB(value float64) float64 {
	if value <= 0.03928 {
		return value / 12.92
	}
	return math.Pow((value+0.055)/1.055, 2.4)
}
