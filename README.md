# bnn

A declarative machine setup tool powered by [mise](https://mise.jdx.dev), with a custom Erlang-flavored DSL.

> **bnn** = banana — terse, fast, and gets the job done.

---

## Philosophy

- Replace Ansible for local dev machine setup
- Declare runtimes and setup steps in a clean DSL
- Idempotent by default via `check` guards
- Extend via `bunch` — self-contained setup units
- Powered by `mise` for runtime management

---

## Installation

```bash
# macOS (Homebrew)
brew tap you/bnn
brew install bnn

# Linux / macOS (curl)
curl -fsSL https://raw.githubusercontent.com/you/bnn/main/install.sh | sh

# Go install
go install github.com/you/bnn@latest
```

---

## Quick Start

Create a `bnn.conf` in your project or home directory:

```erlang
% bnn.conf

bunch(ruby,
    runtime(mise, "3.3"),
    depends([]),
    check("mise current ruby | grep 3.3"),
    steps([
        pre("echo preparing ruby"),
        run("gem install bundler"),
        run("gem install rubocop"),
        post("echo ruby ready")
    ])
).

bunch(node,
    runtime(mise, "22"),
    depends([]),
    check("mise current node | grep 22"),
    steps([
        run("npm install -g pnpm"),
        run("npm install -g typescript")
    ])
).

bunch(rails,
    runtime(shell),
    depends([ruby, node]),
    steps([
        run("gem install rails")
    ])
).
```

Then run:

```bash
bnn apply
```

---

## CLI Commands

| Command | Description |
|---|---|
| `bnn apply` | Apply all bunches |
| `bnn apply ruby` | Apply specific bunch |
| `bnn apply --dry` | Dry run — print what would happen |
| `bnn status` | Show installed vs declared state |
| `bnn check` | Run all check guards, report state |
| `bnn migrate` | Detect rvm/nvm/rbenv and migrate to mise |
| `bnn doctor` | Check prerequisites (mise installed, PATH set) |

---

## DSL Syntax

### Top Level Structure

Every top-level term ends with a period `.`

```erlang
bunch(name, arg1, arg2, ...).
```

### `bunch`

Declares a setup unit.

```erlang
bunch(ruby,
    runtime(mise, "3.3"),
    depends([]),
    check("mise current ruby | grep 3.3"),
    steps([
        pre("echo preparing ruby"),
        run("gem install bundler"),
        post("echo ruby ready")
    ])
).
```

| Arg | Required | Description |
|---|---|---|
| name | yes | atom — identifier for this bunch |
| `runtime(...)` | yes | which tool manages this runtime |
| `depends([])` | no | list of bunches to run before this one |
| `check(...)` | no | if command exits 0 — skip this bunch |
| `steps([...])` | yes | ordered list of commands to execute |

### `runtime`

```erlang
runtime(mise, "3.3")    % mise manages ruby 3.3
runtime(brew)           % homebrew manages this
runtime(shell)          % plain shell, no runtime manager
```

| Atom | Description |
|---|---|
| `mise` | use mise to install and manage version |
| `brew` | use homebrew |
| `shell` | run steps directly in shell |

### `depends`

```erlang
depends([])              % no dependencies
depends([ruby])          % run after ruby bunch
depends([ruby, node])    % run after both
```

List of atoms — unquoted bunch names.

### `check`

Idempotency guard. If the command exits `0`, the entire bunch is skipped.

```erlang
check("mise current ruby | grep 3.3")
check("command -v pnpm")
check("test -f ~/.gitconfig")
```

### `steps`

Ordered list of `pre`, `run`, and `post` terms.

```erlang
steps([
    pre("echo before"),        % runs first
    run("gem install bundler"),% main steps — can have multiple
    run("gem install rubocop"),
    post("echo after")         % runs last
])
```

| Term | Description |
|---|---|
| `pre(cmd)` | runs before all `run` steps — optional |
| `run(cmd)` | main execution — required, multiple allowed |
| `post(cmd)` | runs after all `run` steps — optional |

---

## Token Reference

```
KEYWORD     bunch, runtime, depends, check, steps, pre, run, post
ATOM        ruby, mise, shell, node     unquoted lowercase identifier
STRING      "3.3", "gem install ..."    always quoted
LPAREN      (
RPAREN      )
LBRACKET    [
RBRACKET    ]
COMMA       ,
PERIOD      .                           top level term terminator
COMMENT     %                           rest of line ignored
```

---

## AST Structure

```
ManifestNode
└── BunchNode
    ├── Name        atom       ruby
    ├── Runtime
    │   ├── Type    atom       mise | brew | shell
    │   └── Version string     "3.3"
    ├── Depends     []atom     [node, ruby]
    ├── Check       string     "mise current ruby | grep 3.3"
    └── Steps       []StepNode
        ├── StepNode  pre    "echo preparing ruby"
        ├── StepNode  run    "gem install bundler"
        └── StepNode  post   "echo ruby ready"
```

---

## Execution Pipeline

```
bnn.conf
     ↓
   Lexer        tokenize source
     ↓
  Parser        tokens → AST
     ↓
 Validator      check rules, depends targets exist, runtimes valid
     ↓
  Visitor       walk AST
     ↓
mise install ruby@3.3
mise global  ruby@3.3
gem install bundler
```

---

## Nesting Levels

```
level 0     bunch(...).            top level term
level 1     runtime(...)           arg of bunch
level 1     depends([])            arg of bunch
level 1     check(...)             arg of bunch
level 1     steps([...])           arg of bunch
level 2     pre(...)               item inside steps list
level 2     run(...)               item inside steps list
level 2     post(...)              item inside steps list
```

---

## Syntax Rules

```
1   every top level term ends with .
2   args separated by ,
3   bunch name is always ATOM — no quotes
4   runtime manager is always ATOM — mise brew shell
5   versions are always STRING — "3.3" "22"
6   commands are always STRING — "gem install bundler"
7   depends list contains ATOMs — no quotes on names
8   steps list is ordered — pre runs first, post runs last
9   pre and post are optional — run is required
10  multiple run() allowed — executes in order declared
11  % starts a comment — rest of line ignored
```

---

## Migration from rvm / nvm / rbenv

bnn detects legacy version managers and migrates to mise:

```bash
bnn migrate
```

```
⚠ Found: rvm (ruby) — active version: 3.3.0
→ Installing ruby@3.3.0 via mise ✓
→ Setting ruby@3.3.0 as global ✓

⚠ ACTION REQUIRED: Remove rvm init from shell config
  Look for pattern: \[ -s.*rvm
  In files: ~/.bashrc, ~/.zshrc, ~/.bash_profile

✓ Migration complete
  Restart your shell or run: exec $SHELL
```

Detected managers:

| Manager | Runtime |
|---|---|
| `rvm` | ruby |
| `rbenv` | ruby |
| `chruby` | ruby |
| `nvm` | node |
| `nodenv` | node |
| `fnm` | node |
| `pyenv` | python |

---

## Project Structure

```
bnn/
├── main.go
├── ast/
│   └── ast.go                  AST node definitions
├── internal/
│   └── parser/
│       ├── dsl/
│       │   ├── lexer.go        tokenizer
│       │   ├── parser.go       tokens → AST
│       │   └── parser_test.go
│       └── toml/               legacy TOML parser (v0.1)
├── runner/
│   └── runner.go               mise shell execution
├── visitor/
│   ├── execute.go              walk AST → run commands
│   ├── dryrun.go               walk AST → print only
│   └── validate.go             walk AST → check rules
└── cmd/
    ├── apply.go
    ├── status.go
    ├── migrate.go
    └── doctor.go
```

---

## Prerequisites

- [mise](https://mise.jdx.dev) installed and in PATH
- Go 1.22+ (for building from source)

---

## License

MIT
