package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/logrusorgru/aurora"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

// TaskOut provides a way to pretty print a long running function with
// colors and spinners.
func TaskOut(name string, fn func() error) error {
	isTerm := terminal.IsTerminal(int(os.Stdout.Fd()))

	spin := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	spin.Prefix = fmt.Sprintf("%-40s    ", name)

	if isTerm && log.GetLevel() <= log.InfoLevel {
		// Color restarts the spinner, so this starts it.
		if err := spin.Color("yellow"); err != nil {
			return err
		}
	}

	err := fn()

	result := aurora.Green("\u2713")
	if err != nil {
		result = aurora.Red("\u2718")
	}

	spin.FinalMSG = fmt.Sprintf("%-40s    %s\n", name, result)

	if isTerm && log.GetLevel() <= log.InfoLevel {
		spin.Stop()
	} else {
		fmt.Printf(spin.FinalMSG)
	}

	if err != nil {
		fmt.Printf("%s\t%s\n", aurora.Red("\u21b3"), err)
		return err
	}

	return nil
}
