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
go build -o bnn .
```

Or run directly without building:

```bash
go run . <command>
```

---

## Quick Start

Create a `bnn.conf` in your project or home directory:

```erlang
% bnn.conf

bunch(ruby,
    runtime(mise, "3.3"),
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
✓  bnn.conf found
```

---

## DSL Reference

### Structure

Every top-level term ends with `.`

```erlang
bunch(name, arg1, arg2, ...).
```

### `bunch` arguments

| Argument | Required | Description |
|---|---|---|
| name | yes | identifier for this bunch (unquoted) |
| `runtime(...)` | yes | which tool manages this runtime |
| `depends([...])` | no | bunches that must run before this one |
| `check("cmd")` | no | shell command — if exits 0 the bunch is skipped |
| `steps([...])` | yes | ordered commands to execute |

### `runtime`

```erlang
runtime(mise, "3.3")   % mise installs and manages the version
runtime(brew)          % homebrew manages this tool
runtime(shell)         % no runtime manager, run steps directly
```

### `depends`

```erlang
depends([ruby, node])  % run after ruby and node are done
depends([])            % no dependencies
```

Bunch names — unquoted, no quotes.

### `check`

Idempotency guard. If the command exits `0`, the entire bunch is skipped.

```erlang
check("mise current ruby | grep 3.3")
check("command -v pnpm")
check("test -f ~/.gitconfig")
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

### Comments

```erlang
% this is a comment — rest of line is ignored
```

---

## Execution Pipeline

```
bnn.conf
    ↓
  Lexer       tokenize source
    ↓
 Parser       tokens → AST
    ↓
Validator     rules: runtime valid, depends targets exist,
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

Errors include location and context:

```
[bnn] line 4:7 — expected a bunch declaration, found "foo"
[bnn] bunch 'rails' — depends on 'ruby' which is not declared
[bnn] circular dependency — rails → ruby → rails
[bnn] bunch 'ruby' — steps must contain at least one run() command
```

---

## Project Structure

```
bnn/
├── main.go
├── bnn.conf
├── ast/
│   └── ast.go                   AST node types
├── internal/parser/dsl/
│   ├── lexer.go                 tokenizer
│   ├── parser.go                tokens → AST
│   └── parser_test.go
├── runner/
│   └── runner.go                mise: Install, SetGlobal, Exec
├── visitor/
│   ├── validate.go              rule checker
│   ├── resolve.go               topological sort
│   ├── dryrun.go                print-only walker
│   └── execute.go               AST → mise runner
└── cmd/
    ├── root.go
    ├── apply.go
    ├── status.go
    └── doctor.go
```

---

## License

MIT
