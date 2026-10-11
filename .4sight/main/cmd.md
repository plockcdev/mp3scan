# Main command

## Behavior

- Dispatch to the `silence` or `missing` subcommand.
- With no subcommand, print help and exit successfully.
- Support `--help` at the root to show the available subcommands and root
  options. Help combined with another argument is an error after showing help.
- Support `usage` and `--usage` at the root to print `docs/USAGE.txt`; reject
  additional arguments to either form.

## Output

Root help lists the `silence` and `missing` subcommands, root options, and no
positional arguments. Root usage gives a short overview and examples for both
subcommands.
