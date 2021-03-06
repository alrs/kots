package logger

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/tj/go-spin"
)

type Logger struct {
	spinnerStopCh chan bool
	spinnerMsg    string
	spinnerArgs   []interface{}
	isSilent      bool
}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) Silence() {
	l.isSilent = true
}

func (l *Logger) Initialize() {
	if l.isSilent {
		return
	}

	fmt.Println("")
}

func (l *Logger) Finish() {
	if l.isSilent {
		return
	}

	fmt.Println("")
}

func (l *Logger) Info(msg string, args ...interface{}) {
	if l.isSilent {
		return
	}

	yellow := color.New(color.FgHiYellow)
	yellow.Printf("    ")
	yellow.Println(fmt.Sprintf(msg, args...))
	yellow.Println("")
}

func (l *Logger) ActionWithoutSpinner(msg string, args ...interface{}) {
	if l.isSilent {
		return
	}

	if msg == "" {
		fmt.Println("")
		return
	}

	white := color.New(color.FgHiWhite)
	white.Printf("  • ")
	white.Println(fmt.Sprintf(msg, args...))
}

func (l *Logger) ChildActionWithoutSpinner(msg string, args ...interface{}) {
	if l.isSilent {
		return
	}

	white := color.New(color.FgHiWhite)
	white.Printf("    • ")
	white.Println(fmt.Sprintf(msg, args...))
}

func (l *Logger) ActionWithSpinner(msg string, args ...interface{}) {
	if l.isSilent {
		return
	}

	s := spin.New()

	c := color.New(color.FgHiCyan)
	c.Printf("  • ")
	c.Printf(msg, args...)
	c.Printf(" %s", s.Next())

	l.spinnerStopCh = make(chan bool)
	l.spinnerMsg = msg
	l.spinnerArgs = args

	go func() {
		for {
			select {
			case <-l.spinnerStopCh:
				return
			case <-time.After(time.Millisecond * 100):
				c.Printf("\r")
				c.Printf("  • ")
				c.Printf(msg, args...)
				c.Printf(" %s", s.Next())
			}
		}
	}()
}

func (l *Logger) ChildActionWithSpinner(msg string, args ...interface{}) {
	if l.isSilent {
		return
	}

	s := spin.New()

	c := color.New(color.FgHiCyan)
	c.Printf("    • ")
	c.Printf(msg, args...)
	c.Printf(" %s", s.Next())

	l.spinnerStopCh = make(chan bool)
	l.spinnerMsg = msg
	l.spinnerArgs = args

	go func() {
		for {
			select {
			case <-l.spinnerStopCh:
				return
			case <-time.After(time.Millisecond * 100):
				c.Printf("\r")
				c.Printf("    • ")
				c.Printf(msg, args...)
				c.Printf(" %s", s.Next())
			}
		}
	}()
}

func (l *Logger) FinishChildSpinner() {
	if l.isSilent {
		return
	}

	white := color.New(color.FgHiWhite)
	green := color.New(color.FgHiGreen)

	white.Printf("\r")
	white.Printf("    • ")
	white.Printf(l.spinnerMsg, l.spinnerArgs...)
	green.Printf(" ✓")
	white.Printf("  \n")

	l.spinnerStopCh <- true
	close(l.spinnerStopCh)
}

func (l *Logger) FinishSpinner() {
	if l.isSilent {
		return
	}

	white := color.New(color.FgHiWhite)
	green := color.New(color.FgHiGreen)

	white.Printf("\r")
	white.Printf("  • ")
	white.Printf(l.spinnerMsg, l.spinnerArgs...)
	green.Printf(" ✓")
	white.Printf("  \n")

	l.spinnerStopCh <- true
	close(l.spinnerStopCh)
}

func (l *Logger) FinishSpinnerWithError() {
	if l.isSilent {
		return
	}

	white := color.New(color.FgHiWhite)
	red := color.New(color.FgHiRed)

	white.Printf("\r")
	white.Printf("  • ")
	white.Printf(l.spinnerMsg, l.spinnerArgs...)
	red.Printf(" ✗")
	white.Printf("  \n")

	l.spinnerStopCh <- true
	close(l.spinnerStopCh)
}

func (l *Logger) Error(err error) {
	if l.isSilent {
		return
	}

	c := color.New(color.FgHiRed)
	c.Printf("  • ")
	c.Println(fmt.Sprintf("%#v", err))
}
