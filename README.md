# goup

One go, kept on the channel you pick: go's newest release, a line like 1.26, or one release.

It needs no go to start with, and nothing but itself: the release list and every
checksum come from [go.dev](https://go.dev/dl/), not GitHub or git.

goup is to go what rustup is to rust and dotnetup is to .NET, minus what go already does
itself: a project's `go.mod` picks the toolchain that builds it, and go downloads that one
on its own ([GOTOOLCHAIN](https://go.dev/doc/toolchain)). goup keeps the go you run
everywhere else, and keeps it up to date.

## Install

macOS and Linux:

```sh
curl --fail-with-body --location --no-progress-meter --url https://raw.githubusercontent.com/meop/goup/main/install.sh | sh
```

Windows:

```powershell
irm -ErrorAction Stop -Uri https://raw.githubusercontent.com/meop/goup/main/install.ps1 | iex
```

Then put `~/.goup/bin` on your `PATH`. It holds `goup` and the `go` and `gofmt`
it installs.

## Usage

```sh
goup install         # follow go's newest stable release, as go and gofmt
goup install latest  # the same, said aloud
goup install 1.26    # follow the newest 1.26.x instead
goup install 1.26.4  # stay on exactly 1.26.4
goup default         # the channel go follows, and the release it is on
goup default 1.27    # the same as install 1.27

goup update          # move go to the newest release its channel allows
goup check           # say whether one is out
goup list            # what is installed, and its commands
goup find            # the newest release of each line go has for this machine
goup find 1.26       # every 1.26 release, candidates and betas among them

goup uninstall       # remove go
goup tidy            # remove unused cached archives and orphaned items
goup tidy --all      # remove all cached archives

goup upgrade         # upgrade goup itself
```

There is one go at a time. Installing another channel replaces the one installed, once the
new one is in place. The commands keep their names, `go` and `gofmt`, whatever the channel.

### Global flags

| Flag | Description |
|---|---|
| `--dry-run`, `-n` | Print what would be done without executing |
| `--yes`, `-y` | Skip confirmation prompts |
| `--verbose`, `-v` | Also print the checks and syncs of goup's vendored tools, and more detail on their errors |
| `--non-interactive` | Never prompt; for scripted use |

### Per-command flags

| Flag | Commands | Description |
|---|---|---|
| `--reinstall`, `-r` | `install`, `default`, `update` | Reinstall even if already installed / already at latest version |
| `--force`, `-f` | `install`, `default`, `update` | Replace a file in `~/.goup/bin` that goup did not put there |
| `--all` | `tidy` | Remove all cached archives regardless of installation status |
| `--long-names`, `-l` | `list`, `find`, `check` | Print names only, one per line, and nothing when there are none |
| `--short-names`, `-s` | `list`, `find`, `check` | Print names only, space-separated on one line, and nothing when there are none |

### Channels

The one go follows one channel, the one it was installed with:

| Syntax | Channel | Follows | `goup update` |
|---|---|---|---|
| `goup install` | `go` | go's newest stable release | moves to each new release |
| `goup install 1` | `go@1` | the newest 1.x | moves within 1.x |
| `goup install 1.26` | `go@1.26` | the newest 1.26.x | moves within 1.26.x |
| `goup install 1.26.4` | `go@1.26.4` | exactly 1.26.4 | never moves |

Installing another channel replaces this one; goup never keeps two. `list` and `default`
print the channel's name.

## Behavior

- Nothing is installed until you install it, and `update` only moves what was installed.
- `find` lists only releases with an archive for this machine, which are the
  ones `install` can put here.
- Updating unpacks the new release beside the old, points the commands at it, and only
  then removes the old one. If any step fails, go is put back as it was.
- Every archive is checked against the sha256 go.dev states before it is
  unpacked. It stays in `~/.goup/download` while go is on that release;
  `goup tidy` removes the ones nothing is on.
- The commands in `~/.goup/bin` are small [wedge](https://github.com/meop/wedge)
  shims that run the real ones where they are unpacked, so there are no
  symlinks or junctions.
- A release candidate is installed only when you name it (`goup install 1.28rc1`).
- `goup upgrade` replaces only the goup the install script put in
  `~/.goup/bin`. One installed another way (a package manager, `go install`) works the
  same and keeps the same `~/.goup`, but is upgraded by what installed it.

## Verifying releases

Every release archive contains `LICENSE`. Verify the signed checksum manifest
with the public release key before checking downloaded artifacts:

```sh
gpg --verify SHA256SUMS.sig SHA256SUMS
sha256sum --ignore-missing --check SHA256SUMS
```

## License

[MIT](LICENSE)
