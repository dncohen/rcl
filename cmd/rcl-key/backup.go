// COPYRIGHT(C) 2020  David N. Cohen

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Operation backup
//
// The backup operation's primary function is to show, in your
// terminal, a secret produced by the generate operation. This allows
// you to copy the secret to a paper backup.
//
// The backup operation also produces a `.cfg` file that corresponds
// to an `.rcl-key` file. Note each `.rcl-key` files contains an
// unencrypted secret, and should be handled securely.  The
// corresponding `.cfg` created by this operation contains a public
// address and optional nickname; it does not include a secret key, so
// may be shared and stored less securely.
//
// The `.rcl-key` file must be available to the `rcl-key` command when
// signing transactions.  While the `.cfg` file should be available to
// the `rcl-tx` command when composing transaction.
//
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"
	"src.d10.dev/command"
)

func init() {
	command.RegisterOperation(command.Operation{
		Handler:     opBackup,
		Name:        "backup",
		Syntax:      "backup <filename> [...]",
		Description: "Shows secret keys, providing an opportunity to make paper backup.",
	})
}

func opBackup() error {
	err := command.ParseOperationFlagSet()
	if err != nil {
		return err
	}

	argument := command.OperationFlagSet.Args()
	if len(argument) == 0 {
		return errors.New("operation expects a list of key files")
	}

	scanner := bufio.NewScanner(os.Stdin)

	// expand globs for inferior operating systems
	arg := make([]string, 0, len(argument))
	for _, a := range argument {
		match, err := filepath.Glob(a)
		command.Check(err)
		for _, m := range match {
			arg = append(arg, m)
		}
	}

	for _, filename := range arg {
		k := &Key{}
		err := ReadKeyFromFile(k, filename)
		command.Check(err)

		txt := ""

		clearScreen()

		// TODO(dnc): prompt user to prove they've backed up each word
		fmt.Printf("\nPublic address: %s\n\n", k.Account)
		for txt != "y" && txt != "n" {
			fmt.Print("view secret (y/n)? ")
			scanner.Scan()
			txt = scanner.Text()
		}
		if txt != "y" {
			fmt.Println("exiting")
			break
		}

		fmt.Println("Secret:")
		fmt.Println("\n", k.Secret)
		fmt.Println("") // space makes more readable

		txt = ""

		for txt != "y" && txt != "n" {
			fmt.Print("proceed (y/n)? ")
			scanner.Scan()
			txt = scanner.Text()
		}

		clearScreen()

		if txt != "y" {
			fmt.Println("exiting")
			break
		}

		// save a config file
		nick := k.Nickname
		if nick == "" {
			nick = k.Account.String()
		}

		cfgname := fmt.Sprintf("%s.cfg", nick)
		file, err := os.OpenFile(cfgname, os.O_RDWR|os.O_CREATE, 0600) // writeable so that comments can be added, or renamed
		command.Check(err)
		fmt.Fprintf(file, "[%s]\n", nick)
		fmt.Fprintf(file, "\taddress=%s\n", k.Account)
		file.Close()
		command.Infof("wrote public address %s to file %q\n", k.Account, cfgname)
	}

	return nil
}

// clear mnemonic from screen
func clearScreen() {

	// clear mnemonic from screen
	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		for i := 0; i <= 30; i++ {
			fmt.Println("")
		}
	}
}
