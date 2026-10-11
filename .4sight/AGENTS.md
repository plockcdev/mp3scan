Do not treat CHANGELOG as a spec input

# 4sight spec driven implementation management of program

## DB Specification

The location for db schema spec lives in `.4sight/schemas`, these example files
are all in the `.4sight/schemas` directory:

`dolt_myapp.md` full, current specification for myapp database in a dolt db

## Subcommands

In general the CLI can have nested subcommands, subcommands can have arguments
and flags.

Files related to how the commands work and are built can be found according to
directories in .4sight

- main - for the no args main command
- subcommands/* -  each subdir under `subcommands` has a name matching the subcommand name
and has files related to that named subcommand, and possibly a nested
subcommands dir

if there is no cmd.md for a subcommand dir, then there is no subcommand

## Spec Docs

DB Schema specs are in `.4sight/schemas`, the rest are code implementation
specs. For more info on how to use db schema specs, see migrate design, the rest
of this document is focused on code specs.

The program design is split across design.md files within the subcommand
structure

- `design.md` - design for the entire program
- `coding.md` - coding standards for entire program
- `main/design.md` - design specific to the main command
- `subcommands/*/design.md` - design files for each subcommand

Each subcommand can have subcommands with their own design docs, coding
standards, and subcommands.

The design is inherited from parent design files, so the entire path of design
files have to be read to understand a given subcommand's design.

The design may include db usage, but not SQL DDL or statements.

It is OK to not have a design.md or coding.md within a given command directory
if not needed.

There is a cmd.md that is a peer to the design.md, it gives particular
specification for the cli command like input args, env vars, flags, etc. how it
behaves, and how it outputs. This is code specific.

# Implementing Schema Specs

If user asks you to implement a schema spec or "the schema", run `4sight migrate
implement prompt <path to schema spec relative to schemas dir>` and then follow
the prompt

Do not do this if they just ask to implement "the spec" Do not update any schema
specs or code specs as part of implementation

# Implementing Code Specs

If user asks you to implement code spec (or doesn't specify which kind of spec),
run `4sight spec implement prompt` and then follow the prompt

Do not update or import any code or schema specs during implementation

Before implementing code that depends on a schema change, check that
database’s current and submitted migration symlinks. If they point to
different migrations, stop and ask the user to run 4sight migrate up. Do not
update code to work with the pending schema until current matches submitted.

# Handling User request for spec review

When the user requests a spec review, run `4sight spec change review agent
prompt` and follow the instructions

# Handling spec change requests

When the user requests a change to the spec and provides instructions on those
changes, run `4sight spec change request <prompt>` where prompt is their request
and follow the instructions
