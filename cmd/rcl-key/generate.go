package main

import (
	"bufio"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"
	"github.com/rubblelabs/ripple/data"
)

func (s *State) generate(args ...string) {
	const help = `

Generate new keypairs and addresses for use on the Ripple Consensus
Ledger.

Generated keys are saved (unencrypted) to a file named
'rcl-key-<address>.cfg'.  The file is not encrypted, so handle with
care.

`

	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	fs.Int("n", 1, "Number of keypairs to generate.")
	fs.String("vanity", "", "Optional regular expression to match.")

	// TODO curve

	s.ParseFlags(fs, args, help, "generate [-n=<int>] [-vanity=<regex>]")

	s.generateCommand(fs)

}

type key struct {
	seed    data.Seed
	keyType data.KeyType
	seq     *uint32
	account data.Account
}

func generate(keyType data.KeyType, seq *uint32) (key, error) {

	key := key{
		keyType: keyType,
		seq:     seq,
	}

	seedBytes := make([]byte, len(key.seed))

	n, err := rand.Read(seedBytes)
	if err != nil {
		return key, err
	}
	if n != len(key.seed) {
		// Sanity check.
		return key, fmt.Errorf("Expected %d seed bytes, got %d\n", len(key.seed), n)
	}

	copy(key.seed[:], seedBytes)
	// Now seed has 16 bytes from crypt.Rand.

	secret, err := key.seed.Hash()
	if err != nil {
		return key, err
	}

	key.account = key.seed.AccountId(key.keyType, key.seq)

	// Sanity check
	sanity, err := data.NewSeedFromAddress(secret.String())
	if err != nil {
		return key, err
	}
	if *sanity != key.seed {
		return key, fmt.Errorf("Seed sanity check failure.")
	}

	return key, nil
}

func (s *State) generateCommand(fs *flag.FlagSet) {
	log.SetPrefix(programName + " generate: ")

	vanity := stringFlag(fs, "vanity")

	count := intFlag(fs, "n")
	if count <= 0 {
		s.Exitf("count parameter (%d) should be 1 or more.", count)
	}

	tolerance := 1 * time.Minute // How long to allow without a match.

	/* TODO
		chanCache := 0
		procs := 1
	  cpu := 1
		if count > 1 || vanity != "" {
			// When generating multiple or searching for vanity, spawn multiple threads.
			chanCache = 1000
			procs = runtime.GOMAXPROCS(runtime.NumCPU())
	    cpu = runtime.NumCPU()
		}
	*/

	// We will use goroutines to generate, in case n > 1,
	matched := make(chan key, 0)
	unmatched := matched // if no vanity, we only need one channel.

	if vanity != "" {
		// Only generate addresses that match regexp.
		exp, err := regexp.Compile(vanity)
		if err != nil {
			s.Exitf("Bad vanity expression \"%s\": %s", vanity, err)
		}

		// This channel will hold the keys that may or may not match.
		unmatched = make(chan key, 0)

		// Attempted to match all generated addresses.
		go func(in, out chan key) {
			for {
				select {
				case key := <-in:
					if exp.MatchString(key.account.String()) {
						out <- key
					}
				}
			}
		}(unmatched, matched)
	}

	go func(out chan key) {
		for {
			seq := uint32(0)
			key, err := generate(data.ECDSA, &seq)
			if err != nil {
				log.Panic(err)
			}
			out <- key
		}
	}(unmatched)

	for i := 0; i < count; i++ {
		select {
		case key := <-matched:

			// Save to disk before displaying the address
			filename := fmt.Sprintf("rcl-key-%s.cfg", key.account)
			err := key.save(filename)
			if err != nil {
				log.Panic(errors.Wrapf(err, "Failed to save secret to file \"%s\"", filename))
			} else {
				log.Printf("generated %s, secret saved to %s\n", key.account, filename)
			}

		case <-time.After(tolerance):
			s.Exitf("Waited %s for vanity match. Giving up!", tolerance)
		}

	}

}

func (key key) save(filename string) error {

	hash, err := key.seed.Hash() // "hash" is the secret.
	if err != nil {
		return err // Should never be reached.
	}

	cfg := ini.Empty()
	sec, err := cfg.NewSection(key.account.String())
	if err != nil {
		return err
	}
	sec.NewKey("secret", hash.String())
	if key.keyType == data.ECDSA {
		sec.NewKey("type", "ecdsa")
		sec.NewKey("sequence", fmt.Sprintf("%d", *key.seq))
	} else {
		log.Panicf("key type %s not yet supported", key.keyType)
	}

	// Create read-only file.
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0400)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()
	_, err = cfg.WriteToIndent(w, "\t")
	return err
}
