package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type item struct {
	title, desc string
}

func (i item) FilterValue() string { return i.title }

type listModel struct {
	list   list.Model
	choice string
	quit   bool
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.quit = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = i.title
			}
			return m, tea.Quit

		case "ctrl+n":
			m.list.CursorDown()
			return m, nil

		case "ctrl+p":
			m.list.CursorUp()
			return m, nil

		case "ctrl+a":
			for m.list.Index() > 0 {
				m.list.CursorUp()
			}
			return m, nil

		case "ctrl+e":
			for m.list.Index() < len(m.list.Items())-1 {
				m.list.CursorDown()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m listModel) View() string {
	if m.quit {
		return quitTextStyle.Render("Cancelled.")
	}
	return "\n" + m.list.View()
}

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s", i.title)
	if i.desc != "" {
		str += fmt.Sprintf(" - %s", i.desc)
	}

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, ""))
		}
	}

	fmt.Fprint(w, fn(str))
}

// ContainerInfo represents container information
type ContainerInfo struct {
	Name  string
	Image string
	Index int
}

// TagInfo holds tag name and creation time for sorting
type TagInfo struct {
	Tag       string
	CreatedAt interface{} // Using interface{} to avoid importing time package here
}

// SelectDeployment shows TUI for deployment selection
func SelectDeployment(clientset kubernetes.Interface, namespace string) (string, error) {
	ctx := context.Background()
	deployments, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	items := []list.Item{}
	for _, dep := range deployments.Items {
		items = append(items, item{
			title: dep.Name,
			desc:  fmt.Sprintf("Replicas: %d, Available: %d", *dep.Spec.Replicas, dep.Status.AvailableReplicas),
		})
	}

	const defaultWidth = 80
	const listHeight = 14

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select Deployment"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	m := listModel{list: l}

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}

	if m := result.(listModel); m.choice != "" {
		return m.choice, nil
	}

	return "", fmt.Errorf("no deployment selected")
}

// SelectContainer shows TUI for container selection
func SelectContainer(containers []ContainerInfo) (ContainerInfo, error) {
	items := []list.Item{}
	for _, container := range containers {
		items = append(items, item{
			title: container.Name,
			desc:  fmt.Sprintf("Current image: %s", container.Image),
		})
	}

	const defaultWidth = 80
	const listHeight = 14

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select Container"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	m := listModel{list: l}

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return ContainerInfo{}, err
	}

	if m := result.(listModel); m.choice != "" {
		for _, container := range containers {
			if container.Name == m.choice {
				return container, nil
			}
		}
	}

	return ContainerInfo{}, fmt.Errorf("no container selected")
}

// SelectImageTag shows TUI for image tag selection
func SelectImageTag(currentImage string, tags []string) (string, error) {
	items := []list.Item{}

	// Show current image first
	if currentImage != "" {
		items = append(items, item{
			title: fmt.Sprintf("%s (current)", currentImage),
			desc:  "Currently deployed",
		})
	}

	// Add available tags
	imageName := strings.Split(currentImage, ":")[0]
	for _, tag := range tags {
		fullImage := fmt.Sprintf("%s:%s", imageName, tag)
		if fullImage != currentImage {
			items = append(items, item{
				title: fullImage,
				desc:  fmt.Sprintf("Tag: %s", tag),
			})
		}
	}

	const defaultWidth = 80
	const listHeight = 14

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select Image Tag"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	m := listModel{list: l}

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}

	if m := result.(listModel); m.choice != "" {
		// Remove "(current)"
		choice := strings.Replace(m.choice, " (current)", "", 1)
		return choice, nil
	}

	return "", fmt.Errorf("no image selected")
}

// SelectImageTagWithTimestamp shows TUI for image tag selection with timestamps
func SelectImageTagWithTimestamp(currentImage string, tagInfos []TagInfo) (string, error) {
	items := []list.Item{}

	// Show current image first
	if currentImage != "" {
		items = append(items, item{
			title: fmt.Sprintf("%s (current)", currentImage),
			desc:  "Currently deployed",
		})
	}

	// Add available tags with timestamps
	imageName := strings.Split(currentImage, ":")[0]
	for _, tagInfo := range tagInfos {
		fullImage := fmt.Sprintf("%s:%s", imageName, tagInfo.Tag)
		if fullImage != currentImage {
			var desc string
			// Since we're using interface{} for CreatedAt, we need to handle different types
			desc = fmt.Sprintf("Tag: %s", tagInfo.Tag)
			// Note: Timestamp formatting will be handled by the caller

			items = append(items, item{
				title: fullImage,
				desc:  desc,
			})
		}
	}

	const defaultWidth = 80
	const listHeight = 14

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select Image Tag"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	m := listModel{list: l}

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}

	if m := result.(listModel); m.choice != "" {
		// Remove "(current)"
		choice := strings.Replace(m.choice, " (current)", "", 1)
		return choice, nil
	}

	return "", fmt.Errorf("no image selected")
}

// TUI for rollback confirmation
type confirmModel struct {
	message string
	result  bool
	quit    bool
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.result = true
			return m, tea.Quit
		case "n", "N", "q", "ctrl+c", "esc":
			m.result = false
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.quit {
		return quitTextStyle.Render("Cancelled.")
	}

	return fmt.Sprintf(
		"\n%s\n\n%s",
		titleStyle.Render(m.message),
		helpStyle.Render("Press Y to confirm, N to cancel"),
	) + "\n"
}

// ConfirmRollback shows confirmation dialog for rollback
func ConfirmRollback(message string) bool {
	m := confirmModel{
		message: message,
	}

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return false
	}

	if finalModel := result.(confirmModel); !finalModel.quit {
		return finalModel.result
	}

	return false
}

// TUI for custom image input
type textInputModel struct {
	textInput textinput.Model
	err       error
	quit      bool
}

func (m textInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m textInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if len(m.textInput.Value()) > 0 {
				return m, tea.Quit
			}
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quit = true
			return m, tea.Quit
		}

	case error:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m textInputModel) View() string {
	if m.quit {
		return quitTextStyle.Render("Cancelled.")
	}

	return fmt.Sprintf(
		"\n%s\n\n%s\n\n%s",
		titleStyle.Render("Enter custom image:"),
		m.textInput.View(),
		helpStyle.Render("Press Enter to confirm, Esc to cancel"),
	) + "\n"
}

// InputCustomImage shows text input for custom image entry
func InputCustomImage(placeholder string) (string, error) {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60

	m := textInputModel{
		textInput: ti,
		err:       nil,
	}

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}

	if m := result.(textInputModel); !m.quit && m.textInput.Value() != "" {
		return m.textInput.Value(), nil
	}

	return "", fmt.Errorf("no image entered")
}
