## Operation backup

The backup operation's primary function is to show, in your terminal, a
secret produced by the generate operation. This allows you to copy the
secret to a paper backup.

The backup operation also produces a `.cfg` file that corresponds to an
`.rcl-key` file. Note each `.rcl-key` files contains an unencrypted secret,
and should be handled securely. The corresponding `.cfg` created by this
operation contains a public address and optional nickname; it does not
include a secret key, so may be shared and stored less securely.

The `.rcl-key` file must be available to the `rcl-key` command when signing
transactions. While the `.cfg` file should be available to the `rcl-tx`
command when composing transaction.

## Command rcl-key - Operation generate

Generate new keypairs and addresses for use on the Ripple Consensus Ledger.

Generated keys are saved to a file named '[ADDRESS].rcl-key', where
[ADDRESS] is the address derived from the public key. The file is not
encrypted, so handle with care.

After generating `.rcl-key` files, run the `backup` operation. Doing so
generates a `.cfg` file containing the public address and no secret. It is
this `.cfg` file that other rcl tools commands will read. It is recommended
to keep `.rcl-key` files on an offline, secure machine for signing
transactions. The `.cfg` files can be copied to online machines where
transactions are composed.

## Command rcl-key

The rcl-key command generates keys and signs transactions for the Ripple
Consensus Ledger.

Usage:

    rcl-key [flags...] <operation> [operation flags...]

## Command rcl-key - Operation sign

Sign command expects an encoded unsigned transaction via stdin, and encodes
a signed transaction to stdout.

