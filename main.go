package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type args struct {
	Url string `arg:"required" help:"URL; GET request only"`
	Rps uint   `arg:"required" help:"Request per second"`
}

type attack struct {
	url                  string
	rps                  uint
	totalSuccesses       *int64
	totalFails           *int64
	currentRate          *uint64
	currentRateToDisplay *uint64
	totalSeconds         *int64
}

func newAttack(url string, rps uint) attack {
	var counter uint64
	var current uint64
	var succ int64
	var fail int64
	var sec int64
	return attack{
		url:                  url,
		rps:                  rps,
		currentRate:          &counter,
		currentRateToDisplay: &current,
		totalSuccesses:       &succ,
		totalFails:           &fail,
		totalSeconds:         &sec,
	}
}

func runAttack(attack attack) {
	ticker := time.NewTicker(time.Duration(1_000_000_000 / int64(attack.rps)))
	for {
		select {
		case <-ticker.C:
			go func() {
				*attack.currentRate++
				res, err := http.Get(attack.url)
				if err != nil {
					*attack.totalFails++
					return
				}
				defer res.Body.Close()
				if res.StatusCode >= 200 && res.StatusCode < 300 {
					*attack.totalSuccesses++
				}
				if res.StatusCode >= 400 {
					*attack.totalFails++
				}
			}()
		}
	}
}

func main() {
	var args args
	arg.MustParse(&args)
	if args.Rps == 0 {
		fmt.Println(
			lipgloss.
				NewStyle().
				Background(lipgloss.Color("205")).
				Padding(0, 5).
				Render("rps can't be 0"))
		os.Exit(1)
	}
	attack := newAttack(args.Url, args.Rps)
	p := tea.NewProgram(initialModel(attack))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

type model struct {
	attack attack
	spin   spinner.Model
}

func initialModel(attack attack) model {
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{attack: attack, spin: s}
}

type tickMsg time.Time

func (m model) Init() tea.Cmd {
	go runAttack(m.attack)
	return tea.Batch(
		m.spin.Tick,
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		*m.attack.currentRateToDisplay = *m.attack.currentRate
		*m.attack.currentRate = 0
		*m.attack.totalSeconds++
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	default:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}

	return m, nil
}

var numberStyle = lipgloss.NewStyle().
	Bold(true)

func formatInt(int int64, color string) string {
	return numberStyle.Foreground(lipgloss.Color(color)).Render(strconv.FormatInt(int, 10))
}

func formatUInt(uint uint64, color string) string {
	return numberStyle.Foreground(lipgloss.Color(color)).Render(strconv.FormatUint(uint, 10))
}

func formatFloat(float float64, color string) string {
	return numberStyle.Foreground(lipgloss.Color(color)).
		Render(strconv.FormatFloat(float, 'f', 2, 64))
}

func (m model) View() string {

	s := fmt.Sprint("\n", m.spin.View(), " ", m.attack.url)
	s += fmt.Sprint("\n    Current rate: ", formatUInt(*m.attack.currentRateToDisplay, "#00A5D4"), " req/s")
	s += fmt.Sprintf("\n    Time passed: %s s", formatInt(*m.attack.totalSeconds, "#00A5D4"))
	s += fmt.Sprintf("\n    Total successes: %s", formatInt(*m.attack.totalSuccesses, "#00CC3E"))
	s += fmt.Sprintf("\n    Total fails: %s", formatInt(*m.attack.totalFails, "#CC0000"))

	successRate := 0.0
	var total = float64(*m.attack.totalSuccesses + *m.attack.totalFails)
	if total != 0.0 {
		successRate = (float64(*m.attack.totalSuccesses) / total) * 100.0
	}

	s += fmt.Sprintf("\n    Success rate (%%): %s", formatFloat(successRate, "#00CC3E"))

	s += "\n\nPress q to quit.\n"

	return s
}
