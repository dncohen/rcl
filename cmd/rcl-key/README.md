## Operation backup

View a generated secret, so that it can be copied to paper.

## Command RCL-key - Operation Generate

Generate new keypairs and addresses for use on the Ripple Consensus Ledger.

Generated keys are saved to a file named 'rcl-key-<address>.cfg'. The file
is not encrypted, so handle with care.

## Command rcl-key

The rcl-key command generates keys and signs transactions for the Ripple
Consensus Ledger.

Usage:

    rcl-key [flags...] <operation> [operation flags...]

## Command rcl-key - Operation sign

Sign command expects an encoded unsigned transaction via stdin, and encodes
a signed transaction to stdout.

