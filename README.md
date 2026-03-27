# bnn

A declarative machine setup tool powered by [mise](https://mise.jdx.dev), with a custom Erlang-flavored DSL.

> **bnn** = banana — terse, fast, and gets the job done.

---

## Prerequisites

- [mise](https://mise.jdx.dev) installed and in `PATH`
- Go 1.22+

---

## Build

```bash
git clone https://github.com/ppp3ppj/bnn
cd bnn
make build        # compiles to ./bnn
```

Or run directly without building:

```bash
make run ARGS="apply --dry"
```

---

## Config File

bnn reads from a single config file:

```
~/.config/bnn/bnn.conf      default location
```

Override for a specific invocation:

```bash
bnn --config ~/projects/myapp/bnn.conf apply
```

---

## Quick Start

Create `~/.config/bnn/bnn.conf`:

```erlang
% ~/.config/bnn/bnn.conf

RubyVersion = "3.3".
NodeVersion = "22".

bunch(ruby,
    runtime(mise, RubyVersion),
    check("mise current ruby | grep ~RubyVersion~"),
    steps([
        pre("echo preparing ruby"),
        run("gem install bundler"),
        run("gem install rubocop"),
        post("echo ruby ready")
    ])
).

bunch(node,
    runtime(mise, NodeVersion),
    check("mise current node | grep ~NodeVersion~"),
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

## Commands

### `bnn apply`

Apply all bunches in dependency order.

```bash
bnn apply
```

### `bnn apply <bunch>`

Apply a single bunch by name.

```bash
bnn apply ruby
```

### `bnn apply --dry`

Print every command that would run without executing anything.

```bash
bnn apply --dry
bnn apply ruby --dry
```

Example output:

```
--- bunch: ruby ---
[dry] check: mise current ruby | grep 3.3  (skip bunch if exits 0)
[dry] mise install ruby@3.3
[dry] mise global  ruby@3.3
[dry] pre: echo preparing ruby
[dry] run: gem install bundler
[dry] run: gem install rubocop
[dry] post: echo ruby ready
```

### `bnn status`

Run each bunch's `check` command and show the result.

```bash
bnn status
```

```
ruby            ✓  mise current ruby | grep 3.3
node            ✗  mise current node | grep 22
rails           ?  no check declared
```

- `✓` — check exits 0, bunch already configured
- `✗` — check fails, bunch needs to run
- `?` — no check declared

### `bnn doctor`

Verify prerequisites are in place.

```bash
bnn doctor
```

```
✓  mise found: /home/user/.local/share/mise/bin/mise
✓  bnn.conf found: /home/user/.config/bnn/bnn.conf
```

---

## Development

### Make targets

```bash
make build          # compile → ./bnn
make run ARGS="apply --dry"   # go run without building

make test           # all packages
make test/dsl       # parser + lexer, verbose
make test/visitor   # validator, resolver, dryrun, execute, verbose
make test/cmd       # cobra CLI, verbose

make fmt            # gofmt all source
make vet            # go vet all packages
make clean          # remove ./bnn binary
make help           # list all targets
```

### Debug logging

Activate with `BNN_DEBUG=1` — output goes to **stderr**:

```bash
BNN_DEBUG=1 bnn apply --dry
# or via make:
make debug
```

```
[bnn:debug] parser: var NodeVersion = "22"
[bnn:debug] parser: interpolate ~NodeVersion~ → "22"
[bnn:debug] resolve: order → ruby → node → rails
[bnn:debug] execute: bunch ruby — check: mise current ruby | grep 3.3
[bnn:debug] execute: bunch ruby — check passed, skipping
[bnn:debug] execute: bunch node — check failed, running
[bnn:debug] execute: runtime mise install node@22
[bnn:debug] execute: step run: npm install -g pnpm
```

---

## DSL Reference

### Comments

```erlang
% this is a comment — rest of line is ignored
```

### Variables

Variables follow Erlang's single-assignment rule. Must start with an **uppercase letter** or **underscore**.

```erlang
NodeVersion = "22".              % string literal
Label       = "node-" ++ "22".  % string concatenation → "node-22"
FullLabel   = Label ++ "-lts".  % chained concat → "node-22-lts"
Alias       = NodeVersion.       % copy from another variable
```

**Rules:**
- Must start with uppercase or `_` — `NodeVersion`, `_Height`
- Single-assignment — rebinding the same variable is a parse-time error
- Declare before use — top to bottom, same as Erlang
- `++` only allowed in variable assignments, not inside `steps`/`check`/`runtime`

#### Manifest-level variables

Declared at the top of the file with a `.` terminator. Visible to all bunches.

```erlang
NodeVersion = "22".
```

#### Bunch-local variables

Declared inside a bunch as a comma-separated argument — no `.` terminator. Scoped to that bunch only.

```erlang
bunch(node,
    Version = "22",    % local — gone after this bunch closes
    runtime(mise, Version),
    steps([...])
).
```

**No shadowing** — a bunch-local variable cannot share a name with a manifest-level variable.

**Same name across sibling bunches is fine** — each bunch is its own scope.

```erlang
bunch(node,
    V = "22",
    runtime(mise, V),
    steps([...])
).

bunch(ruby,
    V = "3.3",   % ✓ different bunch, different scope
    runtime(mise, V),
    steps([...])
).
```

### String Interpolation

Use `~VarName~` inside any quoted string to expand a variable's value.

```erlang
NodeVersion = "22".

check("mise current node | grep ~NodeVersion~")
run("echo installing node ~NodeVersion~")
```

- `~~` → literal `~`
- Variable inside `~...~` must start with uppercase or `_`

### String Concatenation `++`

Build strings from parts using `++`. Only allowed in variable assignments.

```erlang
NodeVersion = "22".
Label       = "node-" ++ NodeVersion.   % "node-22"
FullLabel   = Label ++ "-lts".          % "node-22-lts"
```

Use `~Var~` interpolation inside steps/check instead of `++`:

```erlang
% ERROR — ++ not allowed here
run("node-" ++ NodeVersion).

% CORRECT — build in a variable, interpolate in the step
Label = "node-" ++ NodeVersion.
run("echo ~Label~")
```

### `bunch` arguments

| Argument | Required | Description |
|---|---|---|
| name | yes | identifier for this bunch (unquoted atom) |
| `runtime(...)` | yes | which tool manages this runtime |
| `depends([...])` | no | bunches that must run before this one |
| `check("cmd")` | no | shell command — if exits 0 the bunch is skipped |
| `steps([...])` | yes | ordered commands to execute |

### `runtime`

```erlang
runtime(mise, "3.3")   % mise installs and manages the version
runtime(mise, Ver)     % version from a variable
runtime(brew)          % homebrew manages this tool
runtime(shell)         % no runtime manager, run steps directly
```

### `depends`

```erlang
depends([ruby, node])  % run after ruby and node are done
depends([])            % no dependencies
```

### `check`

Idempotency guard. If the command exits `0`, the entire bunch is skipped.

```erlang
check("mise current ruby | grep 3.3")
check("command -v pnpm")
check(MyCheckVar)   % variable reference
```

### `steps`

```erlang
steps([
    pre("echo before"),         % runs first, optional
    run("gem install bundler"), % main work, required (at least one)
    run("gem install rubocop"), % multiple run() allowed
    post("echo after")          % runs last, optional
])
```

---

## Execution Pipeline

```
~/.config/bnn/bnn.conf
    ↓
  Lexer       tokenize source
    ↓
 Parser       resolve variables, interpolation, and ++ at parse time
              tokens → AST (all strings already expanded)
    ↓
Validator     runtime valid, depends targets exist,
              no duplicate names, at least one run(),
              no circular dependencies
    ↓
 Resolve      topological sort by depends
    ↓
 Execute      for each bunch:
                1. run check — skip if exits 0
                2. mise install + mise global  (mise runtime)
                   brew install <name>         (brew runtime)
                3. mise exec -- sh -c <cmd>    (each step)
```

---

## Error Messages

```
[bnn] line 4:7  — expected a variable binding or bunch declaration, found "foo"
[bnn] line 1:12 — "nodeVersion" looks like a variable but variables must start
                  with an uppercase letter or _ (e.g. NodeVersion)
[bnn] line 2:5  — NodeVersion is already bound (single-assignment only)
[bnn] line 3:5  — NodeVersion is already declared at manifest level
                  — local variables cannot shadow global ones
[bnn] line 5:20 — ~Suffix~ is not defined — declare it above with Suffix = "value".
[bnn] line 8:12 — '++' requires string on left side, got atom "node"
[bnn] line 8:12 — '++' is not allowed in run/pre/post — build the string in a variable first:
                  Label = A ++ B.
                  run/pre/post(Label)
[bnn] bunch 'rails' — depends on 'ruby' which is not declared
[bnn] circular dependency — rails → ruby → rails
[bnn] bunch 'ruby' — steps must contain at least one run() command
```

---

## Project Structure

```
bnn/
├── main.go
├── bnn.conf                     local dev config
├── ast/
│   └── ast.go                   AST node types
├── internal/
│   ├── log/
│   │   └── log.go               debug logger (BNN_DEBUG=1)
│   └── parser/dsl/
│       ├── lexer.go             tokenizer
│       ├── parser.go            tokens → AST, variable resolution
│       └── parser_test.go
├── runner/
│   └── runner.go                mise: Install, SetGlobal, Exec
├── visitor/
│   ├── validate.go              rule checker
│   ├── resolve.go               topological sort
│   ├── dryrun.go                print-only walker
│   └── execute.go               AST → mise runner
└── cmd/
    ├── root.go                  --config flag, default ~/.config/bnn/bnn.conf
    ├── apply.go
    ├── status.go
    └── doctor.go
```

---

## License

MIT
