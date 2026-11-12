# AGENTS.md

goup keeps one go, on the channel it was installed with, the way rustup keeps
rust's default toolchain: go itself picks a project's toolchain (GOTOOLCHAIN),
so goup is only the go you run everywhere else. Underneath it is a stripped
down ghpm for the one package go: the pinning, the manifest, the disk layout,
the output and the conventions are ghpm's on purpose, and most of the code
behind them is shared with it (see **golie**), so read ghpm's AGENTS.md for
the reasoning behind anything here that looks unexplained. The verbs are
rustup's and dotnetup's, with no aliases.

## After every code change

Run all four — do not skip any:

```sh
go build -o goup ./cmd/goup
go test ./...
gofmt -w .
golangci-lint run ./...
```

Tests that access goup state must set `GOUP_TEST_HOME` to a fresh temporary
directory (`withHome(t)` in `internal/config`). The store package deliberately
refuses to resolve the real user home from a Go test binary when it is absent.
Never weaken or bypass this guard: `install`, `update`, `uninstall` and `tidy` delete toolchains.

`GOUP_TEST_HOME` also redirects the real binary, which is how to try a change
by hand without touching `~/.goup`.

## Comments

No comment explains why code does what it does where a unit test can show it: the test is the explanation, and it
fails when the reasoning stops holding. A comment is only for what no test can show.

## Keeping this file current

Update this file when a package is added, removed or renamed, when the manifest
or disk layout changes, or when a flow changes. Update `README.md` when
user-visible behavior changes.

## Cross-compile

```sh
GOOS=darwin  GOARCH=arm64 go build -o goup-darwin-arm64      ./cmd/goup
GOOS=linux   GOARCH=arm64 go build -o goup-linux-arm64       ./cmd/goup
GOOS=linux   GOARCH=amd64 go build -o goup-linux-amd64       ./cmd/goup
GOOS=windows GOARCH=arm64 go build -o goup-windows-arm64.exe ./cmd/goup
GOOS=windows GOARCH=amd64 go build -o goup-windows-amd64.exe ./cmd/goup
```

## Publishing

To cut a release: bump the version in `VERSION` and push to `main`. The CI
pipeline (`.github/workflows/pipeline.yaml`) is ghpm's: version check → validate
(fmt + lint) and the tests on every platform and architecture it ships for, each on its own runner → a GPG-signed tag
`v<VERSION>` and `goreleaser release`.
It requires the `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` repository secrets.
Never create the tag by hand.

## Project structure

- `cmd/goup/` — entry point
- `internal/cli/` — subcommands. `sync.go` holds `install`, `default`,
  `update` and the one mutating flow they share (`apply`); `find.go` lists
  go.dev's releases; `list.go` holds `list` and `check`; `remove.go` holds
  `uninstall` and `dropAll`, which install uses to drop the channel it
  replaces; `tidy.go` is ghpm's over `~/.goup`; `state.go` holds what an
  argument is, and what the commands
  ask of the disk; `table.go` holds the messages, the gate and the closing
  failure table; `upgrade.go` is goup upgrading itself
- `internal/config/` — goup's names for golie's manifest, the one package
  (`go`) and its source (`go.dev/dl`), and goup's lock over `~/.goup`
- `internal/godl/` — go.dev's release list: fetch, resolve a pin to a release,
  order releases, pick the ones a term names, pick the archive for a platform.
  The only source of go's releases and checksums, and the only part of goup
  that is not ghpm's shape
- `internal/store/` — where `~/.goup` is; the layout inside it is golie's

## golie

Everything goup shares with ghpm lives in [golie](https://github.com/meop/golie),
a separate module, and that is most of it: `manifest` (the manifest's format,
keys, pin levels and shim names), `layout` (the directories under `~/.goup`),
`journal` (commit and rollback), `tidy` (what is broken or orphaned, and its
removal), `ui` (output and prompts), `fetch`
(the checked download and extraction), `vendored`, `wedge` and `sig` (the
vendored mallet, at wedge's latest release, checked against its signature),
`shim`, `upgrade`, `lock` and `home`. Versions are goup's own (`godl`): go.dev
marks each release stable or not, `latest` and a line pin (`1`, `1.26`) take
the newest stable, a name go.dev lists is taken as given, and `Compare` orders
go's names (a release above its own `rc` and `beta`, `rc10` above `rc9`).
golie knows nothing of go: where ghpm passes a package name, goup passes
`go`. A change to any of those is made there, released there (bump its
`VERSION`), and taken here with `go get github.com/meop/golie@v<VERSION>`.
What is only goup's — go.dev, what an argument is, the commands and what they
print — stays here.

## Where goup differs from ghpm

Each of these is deliberate, and the list is meant to stay this short. Before
adding a flag or a message, find ghpm's and use its shape and its words, with
`toolchain` for `package`.

- **One toolchain.** go picks a project's toolchain itself, so there is no
  use for several side by side. Installing a channel replaces whatever is
  installed, and the shims are always `go` and `gofmt`, never `go@1.26`
- **rustup's verbs.** `install`, `update`, `uninstall`, `check` and
  `default`, as rustup and dotnetup name them, and no aliases: not ghpm's
  verbs, nor short forms of these. `update` and `uninstall` take
  no argument, since there is one go; `install` takes one channel at most
- **No package argument.** The package is always `go`, so install takes only
  the pin: `goup install 1.26` is `ghpm add go@1.26`, and the key is
  `go@1.26` either way. The key itself is accepted too, since `list` prints it
- **A bare `install` follows latest.** ghpm's `add` needs a name; goup's has
  the only one there is, so with no argument it installs the toolchain that
  follows the newest release, as a bare `uv python install` does. `latest`
  says the same thing aloud
- **`find` searches releases, not packages.** ghpm's registry is a list of
  packages; goup's one package has a list of releases, which is what there is
  to find. No argument lists the newest release of each line, as
  `uv python list` does
- **Nothing is chosen during an install.** A go release has one archive per
  platform and a fixed `bin`, so there is no asset prompt, no bin prompt, no
  second gate, and nothing recorded as declined
- **No GitHub for go.** go's releases and checksums are go.dev's, so there is
  no `gh`, no `info` or `download`, and no `--skip-hash-check`.
  `refresh` is ghpm's verb, for the one thing goup takes from GitHub: wedge
- **`doctor` is ghpm's, for goup's own state**: its lock, its vendored wedge,
  the go each version pointer names, `~/.goup/bin` on PATH, and the cache. It
  prints through golie's `doctor`, so its lines are ghpm's: a vendored tool is
  `vendor - wedge`, with the version it holds, as refresh names it

## Manifest and disk layout

Both are ghpm's, through golie. `~/.goup/manifest.json`:

```json
{
  "repo": { "go": "go.dev/dl" },
  "extract": {
    "go@1.26": { "pin": "minor", "version": "1.26.9", "assets": ["go1.26.9.linux-amd64.tar.gz"], "bin": { "go": "bin/go", "gofmt": "bin/gofmt" } }
  }
}
```

There is one entry. Its key is the channel, `go` or `go@<pin>`, which is
where the pin itself is kept; a manifest with more than one entry is read
as is, and the next install of another channel drops the rest. `pin` is the
level the pin holds it to (`latest`,
`major`, `minor`, `fixed`); `version` is the release it is on, without
go.dev's `go` prefix; `bin` maps each shim, named as it sits on disk, to the
command's path inside the extract dir.

- `~/.goup/bin/` — goup and the shims; the one directory that goes on `PATH`
- `~/.goup/extract/<key>/<version>/` — a go tree, its GOROOT, and
  `extract/<key>/version`, a pointer file naming it, which the shims are stamped
  through, so they need not change from one release to the next. An update
  unpacks into `extract/<key>/.<version>.new`, places it and moves the pointer
  (`golie/journal.Place`, `Point`)
- `~/.goup/download/go.dev/dl/<version>/` — the archive, kept while an install
  is on that release and removed by `tidy` once none is
- `~/.goup/vendor/wedge/` — `mallet` and its templates; never on `PATH`

## Key flows

The flow model is ghpm's: resolve what was asked → a **gate table** + confirm
→ progress lines and a `✓ N` line, with a `✗ N toolchain(s) failed`
name/reason table whenever anything failed.

**Arguments** — `install` (and `default` given one) takes a pin whole
(`parsePin`): `latest`, a line, or one release, spelled bare (`1.26`) or as
its key (`go@1.26`; `go` is `latest`). `find` takes the start of a version
(`godl.StartsWith`: `1.2` starts `1.2.1` and `1.2rc1`, never `1.26`), which
`godl.Found` puts to go.dev's releases. `list`, `check`, `update` and
`uninstall` take nothing: there is one go.

**Install** — one argument at most (none is `latest`): already installed on
that channel says so unless `--reinstall` → resolve the pin against go.dev →
**gate table** (name/version/pin/uri/replaces, the last naming the channel it
replaces) + confirm → `apply` → `dropAll` on every other key, keeping the
shims the new one stamped under the same names.

**Default** — no argument: each key with its release and pin level, as
`go@1.26 → 1.26.9 (minor)`. With one: `install`.

**Update** — moves what was installed and nothing else: with nothing installed
it says so. A `fixed` install is said and skipped; the rest: resolve → a newer
release, or any with `--reinstall` → **gate table**
(name/version/update/pin/uri) + confirm → `apply`.

**apply** (one toolchain) — make sure there is a vendored wedge, once per
command → fetch
the archive into the download cache unless a copy with the right sha256 is
there → unpack it beside its extract dir and rename it in → stamp a shim for
every command in the tree's `bin` → write the manifest entry → remove shims
the new release no longer has, and the old extract dir.

The manifest is the commit point, and the steps before it are journalled
(`golie/journal`): a failure rolls them back to the release the manifest still
names. The old extract dir is deleted only after the commit, because that
rollback restores from it.

**Find** — go.dev's full release list, kept to the releases with an archive
for this machine (what `install` can put here). No term: the newest stable
release of each line. A term: every release it starts, newest first,
candidates and betas included, since naming one is how it is installed. A `name`
column shows the installs on a release, and is dropped when there are none.
Names mode prints the versions.

**List / Check** — ghpm's tables: a row per shim
(name/version/pin/uri/type/target, with `update` for check). `uri` is always `go.dev/dl`; it is kept so the tables are
the ones ghpm prints. `check` leaves fixed installs out.

**Uninstall** — gate → `dropAll`: a toolchain's shims, then its extract dir,
then its manifest entry. Nothing here can be rolled back, so the order is the
safety: a shim that will not delete leaves the toolchain fully described for
the retry. Its cached archive is left for `tidy`.

**Tidy** — ghpm's, by `golie/tidy`: broken installs, orphaned shims, orphaned
extract dirs (a crashed unpack's `.<version>.new` among them, and a release a
running go kept from removal) and orphaned
downloads, each shown and confirmed on its own; `--all` empties the download
cache.

**Upgrade** — goup and nothing else. The newest release is read from where
`releases/latest` redirects, the archive is checked against that release's
`SHA256SUMS`, and that against its signature by the meop release key, and the
running binary is replaced (moved aside first on
Windows, which will not overwrite a running executable). Only the goup in
`~/.goup/bin`, where the install script puts it, is replaced, reached directly
or through a symlink. One that runs from anywhere else — ghpm's, `go
install`'s, a build — was put there by something else, which is what upgrades
it: it is refused in one line (`✗ goup: cannot upgrade itself, running from
<path>: upgrade it with what installed it`, off `golie/upgrade`'s `Check`) before the release page is asked
anything. Everything else such a goup does is unchanged: its
state is still `~/.goup` and its shims still go in `~/.goup/bin`, it is only
not in there itself.

**Vendored wedge** — wedge's latest release, kept by golie's `vendored`, never
pinned. Before anything is stamped, `apply` makes sure there is one, and uses
whatever is there with no network: only where there is none (a new machine) is
the latest fetched, said in the same `asset fetching` line as any download.
**`refresh` is the one command that moves it**, for the user to run when they
mean "get what's new"; it says on one line, labelled `vendor - wedge` and in
golie's `vendored.Said` words as every tool that vendors one does, that wedge
is up to date, or what it installed or updated, and on the next that shims
keep the wedge that stamped them until `update --reinstall`. A download of it
is said under the same label. A release is checked before any of it is
used: its archive against its `SHA256SUMS`, and that against its signature by
the meop release key golie carries. Never have `apply` look for a newer
release, and never make wedge part of `upgrade`.

## Line spacing

A blank line means the run has moved to the next step of its flow. Nothing else gets one.

- Exactly one blank line before every prompt: never none, never two.
- A prompt, the answer typed to it, and the output that answer lets through are one step: the output starts on the line
  right below the answer.
- Lines of one kind in a row (downloads, bins found, refresh lines, a script's output) run tight: no blank between them,
  none after them.
- A step never ends itself with a blank line; the step after it asks for one.
- `-y` answers a prompt without showing it; the prompt's blank line still comes, where the prompt would have been.

The rule is the same, word for word, in golie, ghpm, goup, shire, wut and wut-config.

Here: everything prints through golie's `ui`, and `sep()` is its `Break`. The steps are ghpm's: what was asked for, the gate, the work and the `✓` line that closes it, and what failed. `promptConfirm` asks for the confirm's blank under `--yes`.

## Testing on a real host

CI runs the tests on Linux and Windows on x86_64 and arm64, and macOS on arm64. To run them on a machine of your own over ssh:

- Windows: build the test binary anywhere (`GOOS=windows go test -c -o <pkg>.test.exe ./<pkg>/`), copy it over and run
  it there. In PowerShell, quote a test flag whole (`'-test.v=true'`): PowerShell splits a bare `-test.v` at its dot.
- macOS and Windows fold the case of file names: a test that makes two names differing only by case (or a link from
  one to the other) cannot run there, so test the behavior the folding causes instead.
- In a Windows ssh login, OpenSSH's own tools (`ssh`, `ssh-keygen`) can lose their output: run what uses them from the
  desktop session.

## Conventions

- go's releases and checksums come from go.dev only. No GitHub API call
  anywhere: wedge and goup's own upgrade are plain release downloads
- `--reinstall` and `--force` are two flags because they are two things to
  ask for, and neither implies the other. `--reinstall` redoes goup's own
  work — an install already there, a toolchain already at the newest release —
  and touches nothing that is not goup's. `--force` is the only way a shim
  replaces a file in `~/.goup/bin` that `shim.IsShim` does not say goup
  stamped, which is the one destructive thing either command can do. Both
  mean the same on `install` and on `update`, and the same in ghpm
- A flag is one thing to ask for, means the same on every command that has
  it, and is ghpm's when ghpm has it. Do not copy a flag from uv, rustup or
  anything else because it is there: take what is cleanest, and change both
  tools together
- Names mode prints names and nothing else: `--long-names` / `--short-names`
  on `list`, `find` and `check` are output for a script, so every status
  line is silent under them and an empty result is empty output — never
  `no toolchains installed`, which a script would have to recognise by its
  wording. `list` and `check` print keys and `find` prints versions, since
  those are what every other command takes
- Output is ghpm's, word for word where the thing said is the same:
  `no toolchains installed`, `all toolchains are up
  to date`, `<name>: already installed → <version>`,
  `<name>: fixed at <version>, skipping`, `<name>: asset fetching [<asset>]...`,
  `<name>: bin found [<bin>]`, confirms (`install <name>`,
  `update N toolchain(s)`, `uninstall <name>`, `upgrade goup to <version>`), and
  `✓ installed <name> → <version>`, `✓ updated N toolchain(s)`, `✓ uninstalled go`
- Table colours are ghpm's roles: `new` on the version an install or update
  goes to, `old` on the one it leaves (and on the channel an install replaces),
  `info` on the version in `list` and `uninstall`
