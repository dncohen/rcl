## Operation fx

TODO(dnc): documentation

## Command RCL-data

The rcl-data command retrieves historical data from data.ripple.com/v2/...
and displays information about accounts on the Ripple Consensus Ledger.

Each subcommand has a -help flag that explains it in more detail. For
instance

    rcl-data show -help

explains the purpose and usage of the show subcommand.

There is a set of global flags such as -config to specify the configuration
directory, where rcl-data expects to find one or more *.cfg files. These
global flags apply to all subcommands.

Each subcommand has its own set of flags, which if used must appear after
the subcommand name.

For a list of available subcommands and global flags, run

    rcl-data -help

