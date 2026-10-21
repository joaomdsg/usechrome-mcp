# usechrome-mcp

[![pipeline](https://github.com/joaomdsg/usechrome-mcp/actions/workflows/pipeline.yml/badge.svg)](https://github.com/joaomdsg/usechrome-mcp/actions/workflows/pipeline.yml)

An [MCP](https://modelcontextprotocol.io) server that gives an AI agent a real
Chrome or Chromium browser. The agent opens pages, reads them, clicks and types
on them, and looks at them the way a developer does in DevTools: the console,
the network, the page source, computed styles, the viewport and performance.

It is one static Go binary built on [chromedp](https://github.com/chromedp/chromedp).
It talks MCP over stdio and drives the browser over the Chrome DevTools
Protocol, so it needs no Node.js, no WebDriver and no browser extension.

## What an agent can do with it

| To | Use |
| --- | --- |
| open a page, go back, forward or reload | `navigate` |
| read what is on the page, with refs to act on | `snapshot` |
| see the page or one element | `screenshot` |
| click, hover, type, press keys, pick options | `interact` |
| run JavaScript in the page | `eval` |
| read what the page logged and threw | `console` |
| list the requests and read one in full | `network` |
| read the HTML as served, or any script or stylesheet | `source` |
| read the computed styles and box of an element | `styles` |
| emulate a phone or any window size | `viewport` |
| measure web vitals, slow clicks and long frames | `perf` |
| work in several tabs, follow popups | `tabs` |

## Install

With Go 1.26 or newer:

```sh
go install github.com/joaomdsg/usechrome-mcp/cmd/usechrome-mcp@latest
```

Or download a binary for Linux, macOS or Windows, on amd64 or arm64, from the
[releases](https://github.com/joaomdsg/usechrome-mcp/releases). Each release has
checksums, an SBOM per archive and a build provenance attestation, which
`gh attestation verify <file> --repo joaomdsg/usechrome-mcp` checks.

The binary launches Chrome or Chromium from your `PATH`; `-chrome-path` names
another one. The container image, `ghcr.io/joaomdsg/usechrome-mcp`, brings
Debian's Chromium for linux/amd64 and linux/arm64, tagged with each release and
`latest`. It runs Chromium without its sandbox, because Docker's default seccomp
profile denies the user namespaces the sandbox needs; the container takes its
place. `gh attestation verify oci://ghcr.io/joaomdsg/usechrome-mcp:<tag> --repo
joaomdsg/usechrome-mcp` checks the image.

## Add it to Claude Code

```sh
claude mcp add usechrome -- usechrome-mcp
```

Everything after `--` goes to the server. Some variations:

```sh
# for every project, not only this one
claude mcp add --scope user usechrome -- usechrome-mcp

# watch the browser while the agent works
claude mcp add usechrome -- usechrome-mcp -headed

# from the container, with the host's network so localhost is your machine
claude mcp add usechrome -- docker run -i --rm --network host ghcr.io/joaomdsg/usechrome-mcp

# in a browser you started yourself, with your own logins and extensions
google-chrome --remote-debugging-port=9222 --user-data-dir="$HOME/.usechrome-profile"
claude mcp add usechrome -- usechrome-mcp -cdp-url http://127.0.0.1:9222
```

Other MCP clients take the same command in their configuration, for example:

```json
{ "mcpServers": { "usechrome": { "command": "usechrome-mcp" } } }
```

## How it behaves

- The browser starts on the first tool call and lives as long as the server.
  It runs headless unless `-headed` is given.
- Every tool acts on the current tab unless given a `tab` id. The `tabs` tool
  lists the ids, `t1` first. Tabs that pages open are listed too, and
  `interact` reports the tabs an action opened.
- `snapshot` gives each element it shows a ref such as `e12`; `interact` and
  `styles` take that ref or a CSS selector.
- Each browser operation stops after 30 seconds, or `-timeout`. Text results
  stop near 40,000 characters, about the 10,000 tokens at which Claude Code
  warns, and say what they left out.

## Reference

<!-- reference: `make readme` writes this from the flags and the tool schemas; edit those instead -->
### Flags

| Flag | Description |
| --- | --- |
| `-cdp-url string` | attach to a running browser's DevTools URL instead of launching one |
| `-chrome-path string` | browser binary to launch (default: search PATH) |
| `-headed` | show a browser window instead of running headless |
| `-no-sandbox` | disable the Chromium sandbox, for containers without user namespaces |
| `-timeout duration` | limit for each browser operation (default 30s) |
| `-version` | print the version and exit |

### Tools

#### `console`

Read the DevTools console of a tab's current page, oldest first: console calls, uncaught exceptions with their stack, and browser messages such as failed resource loads. It holds what the page logged since it loaded, also before this server attached. Objects are written as JSON. level picks the least severe level to show.

| Argument | Type | Description |
| --- | --- | --- |
| `level` | string: `debug`, `info`, `warning`, `error` | least severe level to show: debug, info, warning or error; default info, as in DevTools |
| `tab` | string | tab id from the tabs tool; default is the current tab |

#### `eval`

Run JavaScript in a tab as in the DevTools console and return the result's type and value. Promises are awaited, and $, $$ and $x work. A thrown error fails the call with its message.

| Argument | Type | Description |
| --- | --- | --- |
| `expression` (required) | string | JavaScript to run, as typed into the DevTools console |
| `tab` | string | tab id from the tabs tool; default is the current tab |

#### `interact`

Act on an element as a person would: click, hover, type, press a key or select an option. Name the element by its ref from the snapshot tool, such as e12, or by a CSS selector. click and hover scroll the element into view and use the mouse at its centre; they fail if it is disabled, hidden or covered. type replaces the element's content key by key, or types at the focus when no element is named. press sends a key to the element, or else to the focus. select picks an option of a <select> by value or label. When the action starts a navigation, the call waits for the new page to load. JavaScript dialogs are accepted and listed, and so are the tabs the action opened.

| Argument | Type | Description |
| --- | --- | --- |
| `action` (required) | string: `click`, `hover`, `type`, `press`, `select` | what to do: click, hover, type, press or select |
| `ref` | string | ref of the element from the snapshot tool, such as e12 |
| `selector` | string | CSS selector of the element, instead of a ref |
| `text` | string | for type: the text, which replaces the element's content; empty clears it |
| `key` | string | for press: a key such as Enter, Tab, ArrowDown or Control+a |
| `option` | string | for select: the value or label of the option to pick |
| `tab` | string | tab id from the tabs tool; default is the current tab |

#### `navigate`

Open a URL in a tab, or go back, forward or reload. Returns the final URL after redirects, the HTTP status and the title.

| Argument | Type | Description |
| --- | --- | --- |
| `action` | string: `goto`, `back`, `forward`, `reload` | goto (default), back, forward or reload |
| `url` | string | address to open; required for goto |
| `tab` | string | tab id from the tabs tool; default is the current tab |

#### `network`

List the network requests of a tab's current page, oldest first, as the DevTools Network panel shows them: a line per request with its id, method, status, type, MIME type, bytes transferred, time and URL, and why it failed. A redirect is a line per hop. Pass request with an id to see its headers and bodies. Requests are recorded from when this server opened the tab, which keeps its last 1000.

| Argument | Type | Description |
| --- | --- | --- |
| `filter` | string | show only the requests whose line contains this text, ignoring case: part of a URL, a type such as fetch, a status such as 404, or failed |
| `request` | string | id of a request in the list, such as n3, to see its headers and bodies |
| `tab` | string | tab id from the tabs tool; default is the current tab |

#### `perf`

Measure a tab's current page since it began to load: TTFB, FCP, LCP, CLS and INP rated by the Core Web Vitals thresholds, the element of the largest paint, the elements that layout shifts moved, the slowest interactions split into input delay, handlers and rendering, the long animation frames with the scripts that ran in them, the page's element count and the JS heap. Interact with the page first to measure how fast it responds.

| Argument | Type | Description |
| --- | --- | --- |
| `tab` | string | tab id from the tabs tool; default is the current tab |

#### `screenshot`

Capture what a tab shows as an image: the viewport, the whole page with fullPage, or one element with selector. The image is in device pixels, so set scale with the viewport tool to trade sharpness for size.

| Argument | Type | Description |
| --- | --- | --- |
| `fullPage` | boolean | capture the whole scrollable page instead of the viewport |
| `selector` | string | CSS selector of one element to capture instead of the viewport |
| `format` | string: `png`, `jpeg` | png (default) or jpeg |
| `quality` | integer | jpeg quality from 0 to 100; ignored for png |
| `tab` | string | tab id from the tabs tool; default is the current tab |

#### `snapshot`

Outline what a tab shows as assistive technology sees it: roles, names, values and states, one element per line, with a ref such as e12 that the interact tool takes. Hidden elements are left out. Take a new snapshot after the page changes; refs of elements that are gone stop working.

| Argument | Type | Description |
| --- | --- | --- |
| `selector` | string | CSS selector of the element to outline instead of the whole page |
| `tab` | string | tab id from the tabs tool; default is the current tab |

#### `source`

Read the source of a tab's current page as the server sent it, before any script changed it, as view-source and the DevTools Sources panel show it. Pass url to read a script, stylesheet, frame or other file the page loaded instead; the network tool lists them. A long source comes in parts: pass the offset that the note at its end gives to read on.

| Argument | Type | Description |
| --- | --- | --- |
| `url` | string | URL of a script, stylesheet, frame or other file the page loaded; default is the page's own document |
| `offset` | integer | where to start reading, as the note at the end of a cut source gives it |
| `tab` | string | tab id from the tabs tool; default is the current tab |

#### `styles`

Read an element's CSS as the DevTools Styles and Computed panes show it: the rules that match it, highest priority first, with their stylesheet URL and line, its box, and computed values. Name the element by its ref from the snapshot tool, such as e12, or by a CSS selector. Without properties, it computes those the rules set plus basics such as display, position, width, margin, color and font.

| Argument | Type | Description |
| --- | --- | --- |
| `ref` | string | ref of the element from the snapshot tool, such as e12 |
| `selector` | string | CSS selector of the element, instead of a ref |
| `properties` | array of string | CSS properties to compute, such as gap or line-height; default is those the rules set plus layout basics |
| `tab` | string | tab id from the tabs tool; default is the current tab |

#### `tabs`

List the browser's tabs, oldest first, with the tabs that pages or a person opened, after opening, selecting or closing one. The other tools act on the current tab unless given a tab id. open loads url in a new tab and makes it current. select makes a tab current and brings it to the front, since a tab in the background slows its timers. close shuts a tab; when it was current, the newest remaining tab becomes current. The last tab cannot be closed.

| Argument | Type | Description |
| --- | --- | --- |
| `action` | string: `list`, `open`, `select`, `close` | list, the default, open, select or close |
| `url` | string | for open: the page the new tab loads; absent opens a blank tab |
| `tab` | string | for select and close: the tab id; close defaults to the current tab |

#### `viewport`

Read or change a tab's viewport, as DevTools device mode does. With no arguments it reports the size the page sees. width, height, scale and mobile replace any earlier override, which lasts across navigations until reset. With mobile, a page without a meta viewport tag lays out 980px wide, as on a phone, and touch events reach pages loaded afterwards.

| Argument | Type | Description |
| --- | --- | --- |
| `width` | integer | CSS pixels; absent keeps the window's width |
| `height` | integer | CSS pixels; absent keeps the window's height |
| `scale` | number | device pixel ratio, 2 or 3 for a high-density phone; absent keeps the screen's |
| `mobile` | boolean | emulate a phone: meta viewport tag, overlay scrollbars and touch |
| `reset` | boolean | drop the override and return to the window's own size |
| `tab` | string | tab id from the tabs tool; default is the current tab |
<!-- end of reference -->

## How changes ship

There is no code review. Every commit to `main` goes through the
[pipeline](.github/workflows/pipeline.yml), and the pipeline is the only gate:
what a reviewer would check, it checks, and it fails the build when a check
fails.

1. **Commit stage**, three jobs in parallel, each limited to 5 minutes:
   - `commit / test`: build, `go mod tidy`, `go vet`, unit tests with
     `-race`, coverage at least 80% in total and 70% in each package, and
     this README's reference matching the code.
   - `commit / lint`: [golangci-lint](.golangci.yml) with these limits per
     function:
     cyclomatic complexity 10, cognitive complexity 15, 60 lines or 40
     statements, nesting complexity 5, maintainability index 20, no
     duplicate blocks of 150 tokens, and a doc comment on everything exported.
   - `commit / audit`: govulncheck, gitleaks over the whole history,
     actionlint, `goreleaser check` on the release configuration, and commit
     messages of one imperative line of at most 72 characters.
2. **Acceptance stage**, two jobs, both against the binary the commit stage
   built: `acceptance / binary` runs it over stdio against real headless
   Chromium and local fixture pages; `acceptance / image` runs the same tests
   against it inside the release image. The tests speak only MCP, through a
   small domain language in [acceptance/dsl](acceptance/dsl).
3. **Release stage**: a `v*` tag runs the stages above, then
   [GoReleaser](.goreleaser.yaml) builds the binaries and the image and
   publishes them with checksums, SBOMs and signed provenance. Go builds are
   reproducible, so the release fails unless its linux/amd64 build is
   byte-identical to the binary the acceptance stage tested.

The linter also holds the architecture in place. The tools see the browser only
through the plain types in `internal/browser`. The chromedp adapter in
`internal/chrome` knows nothing of MCP. Only `main` wires the two together. The
acceptance tests cannot import the code they test.

## Develop

You need Go 1.26 and Chrome or Chromium. The Makefile runs the same checks, with
the same pinned tool versions, as the pipeline:

```sh
make commit-stage      # build, test, coverage, lint, vulnerabilities, secrets, workflows, release config
make acceptance        # the acceptance tests against dist/usechrome-mcp
make acceptance-image  # the same tests inside the release image; needs Docker
make readme            # rewrite the reference above after changing a flag or a tool
```
