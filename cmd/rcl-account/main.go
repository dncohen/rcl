package main

import (
	"encoding/base64"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dncohen/rcl/cfg"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"

	"upspin.io/shutdown"
)

// An upspin-inspired command with subcommands.  This manipulates
// Ripple Consensus Ledger transactions.

type State struct {
	Name     string // Name of the subcommand we are running.
	ExitCode int    // Exit with non-zero status for minor problems.
}

const intro = `

The rcl-account command displays information about accounts on the Ripple Consensus Ledger.

Each subcommand has a -help flag that explains it in more detail.  For instance

  rcl-account show -help

explains the purpose and usage of the show subcommand.

There is a set of global flags such as -config to specify the
configuration directory, where rcl-tx expects to find one or more
*.cfg files.  These global flags apply to all subcommands.

Each subcommand has its own set of flags, which if used must appear
after the subcommand name.

For a list of available subcommands and global flags, run

  rcl-account -help


`

const (
	LedgerSequenceInterval = 10
	programName            = "rcl-account"
)

var commands = map[string]func(*State, ...string){
	"monitor": (*State).monitor,
	"show":    (*State).show,
}

var config cfg.Config

var (
	// Values useful for RCL math.
	zeroNative    *data.Value
	zeroNonNative *data.Value
	oneNative     *data.Value
	oneNonNative  *data.Value
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())

	zeroNative, _ = data.NewNativeValue(0)
	zeroNonNative, _ = data.NewNonNativeValue(0, 0)
	oneNative, _ = data.NewNativeValue(1)
	oneNonNative, _ = data.NewNonNativeValue(1, 0)
}

// Parse the list of accounts on the command line.
func accountsFromArgs(args []string) (map[string]*data.Account, error) {
	if len(args) > 0 {
		accounts = make(map[string]*data.Account)

		// Each arg could be either address or nickname
		for _, arg := range args {
			account, _, ok := config.GetAccountByNickname(arg)
			if !ok {
				var err error
				account, err = data.NewAccountFromAddress(arg)
				if err != nil {
					return accounts, errors.Wrapf(err, "Bad account address: %s", arg)
				}
			}
			accounts[arg] = account
		}
		return accounts, nil
	} else {
		// TODO: return all known accounts in config.
		return nil, fmt.Errorf("no account specified")
		//return config.GetAccountsByNickname(), nil
	}
}

func main() {
	state, args, ok := setup(flag.CommandLine, os.Args[1:])
	if !ok || len(args) == 0 {
		help()
	}
	if args[0] == "help" {
		help(args[1:]...)
	}
	state.run(args)
	state.ExitNow()
}

// setup initializes the upspin command given the full command-line argument
// list, args. It applies any global flags set on the command line and returns
// the initialized State and the arg list after the global flags, starting with
// the subcommand that will be run.
func setup(fs *flag.FlagSet, args []string) (*State, []string, bool) {
	log.SetFlags(0)
	log.SetPrefix(programName + ": ")
	fs.Usage = usage
	var err error
	// Flags on primary command (as opposed to sub-command)
	var configPath string
	fs.StringVar(&configPath, "config", ".", "Directory containing configuration files")

	err = fs.Parse(args)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	config, err = cfg.LooseLoadGlob(filepath.Join(configPath, "*.cfg"))
	if err != nil && err != cfg.FileNotFound {
		fmt.Println(err)
		os.Exit(2)
	}

	if len(fs.Args()) < 1 {
		return nil, nil, false
	}
	state := newState(strings.ToLower(fs.Arg(0)))
	state.init()

	return state, fs.Args(), true
}

// run runs a single command specified by the arguments, which should begin with
// the subcommand.
func (state *State) run(args []string) {
	cmd := state.getCommand(args[0])
	cmd(state, args[1:]...)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", programName)
	fmt.Fprintf(os.Stderr, "\t%s [globalflags] <command> [flags]\n", programName)
	printCommands()
	fmt.Fprintf(os.Stderr, "Global flags:\n")
	flag.PrintDefaults()
}

// help prints the help for the arguments provided, or if there is none,
// for the command itself.
func help(args ...string) {
	// Find the first non-flag argument.
	cmd := ""
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			cmd = arg
			break
		}
	}
	if cmd == "" {
		fmt.Fprintln(os.Stderr, intro)
	} else {
		// Simplest solution is re-execing.
		command := exec.Command("upspin", cmd, "-help")
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		command.Run()
	}
	os.Exit(2)
}

// printCommands shows the available commands.
func printCommands() {
	fmt.Fprintf(os.Stderr, "Transaction commands:\n")
	var cmdStrs []string
	for cmd := range commands {
		cmdStrs = append(cmdStrs, cmd)
	}

	// There may be dups; filter them.
	prev := ""
	for _, cmd := range cmdStrs {
		if cmd == prev {
			continue
		}
		prev = cmd
		fmt.Fprintf(os.Stderr, "\t%s\n", cmd)
	}
}

// getCommand looks up the command named by op.
// If it's in the commands tables, we're done.
// If the command can't be found, it exits after listing the
// commands that do exist.
func (s *State) getCommand(op string) func(*State, ...string) {
	fn := commands[op]
	if fn != nil {
		return fn
	}
	printCommands()
	s.Exitf("no such command %q", op)
	return nil
}

// newState returns a State with enough initialized to run exit, etc.
// It does not contain a Config.
func newState(name string) *State {
	s := &State{
		Name: name,
	}
	return s
}

// init initializes the State with what is required to run the subcommand,
// usually including setting up a Config.
func (s *State) init() {

	return
}

// ExitNow terminates the process with the current ExitCode.
func (s *State) ExitNow() {
	shutdown.Now(s.ExitCode)
}

// Exitf prints the error and exits the program.
// If we are interactive, it calls panic("exit"), which is intended to be recovered
// from by the calling interpreter.
// We don't use log (although the packages we call do) because the errors
// are for regular people.
func (s *State) Exitf(format string, args ...interface{}) {
	format = fmt.Sprintf("%s: %s\n", s.Name, format)
	fmt.Fprintf(os.Stderr, format, args...)
	s.ExitCode = 1
	s.ExitNow()
}

// Exit calls s.Exitf with the error.
func (s *State) Exit(err error) {
	s.Exitf("%s", err)
}

// ParseFlags parses the flags in the command line arguments,
// according to those set in the flag set.
func (s *State) ParseFlags(fs *flag.FlagSet, args []string, help, usage string) {
	helpFlag := fs.Bool("help", false, "print more information about the command")
	usageFn := func() {
		fmt.Fprintf(os.Stderr, "Usage: %s %s\n", programName, usage)
		if *helpFlag {
			fmt.Fprintln(os.Stderr, help)
		}
		// How many flags?
		n := 0
		fs.VisitAll(func(*flag.Flag) { n++ })
		if n > 0 {
			fmt.Fprintf(os.Stderr, "Flags:\n")
			fs.PrintDefaults()
		}
	}
	fs.Usage = usageFn
	err := fs.Parse(args)
	if err != nil {
		s.Exit(err)
	}
	if *helpFlag {
		fs.Usage()
		os.Exit(2)
	}
}

// IntFlag returns the value of the named integer flag in the flag set.
func intFlag(fs *flag.FlagSet, name string) int {
	return fs.Lookup(name).Value.(flag.Getter).Get().(int)
}

// BoolFlag returns the value of the named boolean flag in the flag set.
func boolFlag(fs *flag.FlagSet, name string) bool {
	return fs.Lookup(name).Value.(flag.Getter).Get().(bool)
}

// StringFlag returns the value of the named string flag in the flag set.
func stringFlag(fs *flag.FlagSet, name string) string {
	return fs.Lookup(name).Value.(flag.Getter).Get().(string)
}

func float64Flag(fs *flag.FlagSet, name string) float64 {
	return fs.Lookup(name).Value.(flag.Getter).Get().(float64)
}

// Probably belong in util.
func decodeInput() (*data.Transaction, error) {
	// GOBs are tricksy.
	// Decode into interface.
	var tx data.Transaction

	encoding := "gob64" // TODO make others optional, if needed.

	// Register instances of what we accept.
	// Should have all tx types here, I am putting them here when needed.
	gob.Register(&data.AccountSet{})
	gob.Register(&data.OfferCreate{})
	gob.Register(&data.Payment{})
	gob.Register(&data.TrustSet{})

	var err error
	// NOTE currently only "gob64" is tested/working/used, and barely.
	if encoding == "gob64" {
		b64Reader := base64.NewDecoder(base64.StdEncoding, os.Stdin)
		err = gob.NewDecoder(b64Reader).Decode(&tx) // Decode into *pointer* to interface.
	} else if encoding == "gob" {
		err = gob.NewDecoder(os.Stdin).Decode(&tx)
	} else {
		err = fmt.Errorf("Unexpected encoding: %s\n", encoding)
	}

	if err != nil {
		return nil, err
	}

	return &tx, nil
}

func encodeOutput(tx *data.Transaction, fs *flag.FlagSet, f *os.File) error {
	if f == nil {
		f = os.Stdout
	}
	encoding := "gob64" // currently only method supported

	var err error
	if encoding == "gob64" {
		// GOB is preferable as it preserves the type of the tx we've created.
		// However it is not terminal safe.  So we further encode to base64.
		b64Writer := base64.NewEncoder(base64.StdEncoding, f)
		defer b64Writer.Close()                    // Close() is important!!
		err = gob.NewEncoder(b64Writer).Encode(tx) // Encode a *pointer* to the interface.
	} else {
		err = fmt.Errorf("Unexpected encoding: %s", encoding)
	}
	return err
}
