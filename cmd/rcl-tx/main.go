package main

import (
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
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
	"github.com/golang/glog"
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
 
The rcl-tx command composes transactions for the Ripple Consensus
Ledger.

Each subcommand has a -help flag that explains it in more detail.  For
instance

  rcl-tx sell -help

explains the purpose and usage of the sell subcommand.

There is a set of global flags such as -config to specify the
configuration directory, where rcl-tx expects to find one or more
*.cfg files.  These global flags apply to all subcommands.

Each subcommand has its own set of flags, which if used must appear
after the subcommand name.

For a list of available subcommands and global flags, run

  rcl-tx -help

`

const (
	LedgerSequenceInterval = 10
	programName            = "rcl-tx"
)

var commands = map[string]func(*State, ...string){
	// Commenting these out for RC exercise.  Not relevant.
	"monitor": (*State).monitor,
	"save":    (*State).save,
	"submit":  (*State).submit,

	// Transactions composers
	"cancel": (*State).cancel, // OfferCancel
	"sell":   (*State).sell,   // OfferCreate
	"send":   (*State).send,   // simple Payment
	"trust":  (*State).trust,  // TrustSet
}

var (
	asAccount *data.Account // The originator when constructing transactions.
	asTag     *uint32

	// Configuration, parsed from .cfg file(s)
	config cfg.Config

	// Helpers for value math
	one data.Value
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())

	tmp, err := data.NewNonNativeValue(1, 0)
	if err != nil {
		log.Panic(err)
	}
	one = *tmp
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

// setup initializes the command given the full command-line argument
// list, args. It applies any global flags set on the command line and returns
// the initialized State and the arg list after the global flags, starting with
// the subcommand that will be run.
func setup(fs *flag.FlagSet, args []string) (*State, []string, bool) {
	log.SetFlags(0)
	log.SetPrefix(programName + ": ")
	fs.Usage = usage
	var err error
	// Flags on primary command (as opposed to sub-command)
	asAddress := fs.String("as", "", "Construct transactions as this address.")
	configPath := fs.String("config", ".", "Directory containing configuration files")

	err = fs.Parse(args)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	config, err = cfg.LooseLoadGlob(filepath.Join(*configPath, "*.cfg"))
	if err != nil && err != cfg.FileNotFound {
		fmt.Println(err)
		os.Exit(2)
	}

	// Default -as <address> from configuration file.
	if asAddress == nil || *asAddress == "" {
		tmp := config.GetAccount()
		asAddress = &tmp
	}
	// Honor -as <address> flag.
	if asAddress != nil && *asAddress != "" {
		asAccount, asTag, err = config.AccountFromArg(*asAddress)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}

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

// usageAndExit prints usage message from provided FlagSet,
// and exits the program with status code 2.
func usageAndExit(fs *flag.FlagSet) {
	fs.Usage()
	os.Exit(2)
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
		command := exec.Command(programName, cmd, "-help")
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
	glog.Flush() // We use glog, and rubblelabs library uses glog.

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

// deprecated in favor of marshal helper package. XXX
func registerTypes() {
	// Register instances of what we accept.
	// Should have all tx types here, I am putting them here when needed.
	// TODO move to shared helper.
	gob.Register(&data.AccountSet{})
	gob.Register(&data.OfferCancel{})
	gob.Register(&data.OfferCreate{})
	gob.Register(&data.Payment{})
	gob.Register(&data.TrustSet{})
}

func decodeInput() (*data.Transaction, error) {
	// GOBs are tricksy.
	// Decode into interface.
	var tx data.Transaction

	encoding := "gob64" // TODO make others optional, if needed.
	registerTypes()

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

// Encode a transaction to JSON.  A helper function for debug output
// and saving to file.  Note that when in pipeline, transactions
// should be encoded and decode by the util/marshal helper package.
func encodeJSON(tx *data.Transaction, f *os.File) error {
	var err error
	if f == nil {
		f = os.Stdout
	}

	j, _ := json.MarshalIndent(tx, "", "\t")
	_, err = fmt.Fprintln(f, string(j))
	return err
}
