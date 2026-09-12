// Command ucode runs the ucode platform on your machine.
//
//	ucode start    bring the stack up, create the first admin, open a browser
//	ucode stop     stop the stack, keep the data
//	ucode status   what is running
//	ucode logs     follow the logs
//	ucode reset    stop the stack and delete all local data
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:           "ucode",
		Short:         "Run the ucode platform locally",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(startCmd(), stopCmd(), statusCmd(), logsCmd(), resetCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s\n", err)
		os.Exit(1)
	}
}

func startCmd() *cobra.Command {
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Bring the stack up and open the admin panel",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := newStack()
			if err != nil {
				return err
			}

			if err := preflight(s.running()); err != nil {
				return err
			}

			step("Preparing %s", s.dir)
			if err := s.materialise(); err != nil {
				return err
			}

			step("Starting containers (first run pulls images, this takes a few minutes)")
			if err := s.run("up", "--detach", "--wait"); err != nil {
				return fmt.Errorf("the stack did not come up.\nRun \"ucode logs\" to see why")
			}

			step("Waiting for services")
			if err := waitReady(3 * time.Minute); err != nil {
				return err
			}

			if !s.bootstrapped() {
				step("Creating the first company and admin")
				switch err := bootstrap(5 * time.Minute); {
				case errors.Is(err, errAlreadyBootstrapped):
					// The platform is set up; only the marker was missing.
				case err != nil:
					return err
				}
				step("Checking that the admin can sign in")
				if err := verifySetup(2 * time.Minute); err != nil {
					return err
				}

				if err := s.markBootstrapped(); err != nil {
					return err
				}

				step("Adding a demo project")
				// An empty workspace is a poor first impression, but a missing
				// demo is not a reason to fail a working installation.
				if err := seedDemo(); err != nil {
					fmt.Fprintf(os.Stderr, "  could not add the demo data: %v\n", err)
				}
			}

			printReady()
			if !noBrowser {
				openBrowser(adminURL)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "do not open a browser")
	return cmd
}

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the stack, keeping all data",
		RunE: func(*cobra.Command, []string) error {
			s, err := newStack()
			if err != nil {
				return err
			}
			return s.run("stop")
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show what is running",
		RunE: func(*cobra.Command, []string) error {
			s, err := newStack()
			if err != nil {
				return err
			}
			return s.run("ps", "--all")
		},
	}
}

func logsCmd() *cobra.Command {
	var follow bool
	var tail int

	cmd := &cobra.Command{
		Use:   "logs [service]",
		Short: "Show the logs of the whole stack or one service",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			s, err := newStack()
			if err != nil {
				return err
			}

			// Printing and exiting is the default so this is usable from a
			// script or a CI step; --follow is the interactive case.
			flags := []string{"logs", "--tail", strconv.Itoa(tail)}
			if follow {
				flags = append(flags, "--follow")
			}
			return s.run(append(flags, args...)...)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "keep printing new lines")
	cmd.Flags().IntVar(&tail, "tail", 100, "how many lines to show per service")
	return cmd
}

func resetCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Stop the stack and delete all local data",
		RunE: func(*cobra.Command, []string) error {
			s, err := newStack()
			if err != nil {
				return err
			}

			if !yes {
				fmt.Println("This deletes every project, table and record in your local ucode.")
				fmt.Print("Type \"reset\" to confirm: ")
				var answer string
				_, _ = fmt.Scanln(&answer)
				if answer != "reset" {
					return fmt.Errorf("cancelled")
				}
			}

			if err := s.run("down", "--volumes"); err != nil {
				return err
			}
			if err := os.Remove(s.markerPath()); err != nil && !os.IsNotExist(err) {
				return err
			}

			fmt.Println("\nLocal data removed. \"ucode start\" will set everything up again.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func step(format string, args ...any) {
	fmt.Printf("→ "+format+"\n", args...)
}

func printReady() {
	fmt.Printf(`
ucode is running.

  Admin panel   %s
  API           %s
  Auth          %s

  Login         %s
  Password      %s

The stack listens on 127.0.0.1 only. Change the password before exposing it
anywhere. "ucode stop" keeps your data; "ucode reset" deletes it.
`, adminURL, gatewayURL, authURL, adminLogin, adminPassword)
}
