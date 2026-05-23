# OpenBao Observability documentation style guide

This guide tells you how to write documentation for the OpenBao Observability
reference architecture project. Follow it for every page you add to this repo,
including how-tos, runbooks, reference pages, design notes, and READMEs.

## 1. Scope and how to use this guide

This guide covers three things:

- Prose: voice, tone, person, tense, sentence patterns.
- Document structure and Markdown conventions.
- Code samples, configuration snippets, and diagrams.

For anything this guide does not specify, follow the
[Google developer documentation style guide][google-style]. If you find a
conflict between this guide and Google's, this guide wins; raise an issue so the
conflict can be resolved or documented.

Terminology and naming conventions (for example, when to say *OpenBao* versus
*Vault*, how to capitalize ecosystem products, banned terms) are out of scope
for this document and will live in a separate terminology guide. Until that
guide exists, mirror the terms used in
[`workstreams/openbao-observability-reference.md`](./openbao-observability-reference.md).

### What the reference architecture is, and what it is not

`openbao-observability-reference.md` is research input. Treat it as the source
of truth for architectural decisions, signal contracts, and citations to
upstream documentation. Do not treat its third-person declarative voice as a
style example. Downstream documentation in this repo is written in the
instructional voice described below.

## 2. Voice and tone

Write to an operator who is configuring, running, or debugging an OpenBao
observability deployment. Assume they are technically capable, short on time,
and reading the page to do something specific.

### 2.1 Use second person

Address the reader as *you*. Do not write *we*, *the user*, *the operator*, or
*one*.

- Good: You configure the metrics token with `read` and `list` on `sys/metrics`.
- Avoid: We recommend that the operator configure a metrics token.

Use *we* only when describing a decision the project made and the reader cannot
change. Keep it rare.

- Acceptable: We chose Grafana Alloy over Promtail because Promtail is EOL on
  2026-03-02.

### 2.2 Use the imperative for steps

Procedures are imperative. Each step is one action.

- Good: Create a Kubernetes Secret named `openbao-metrics-token`.
- Avoid: The user should now create a Kubernetes Secret.

### 2.3 Use active voice

Name the actor. Passive voice hides who does what and is harder to scan.

- Good: Prometheus scrapes `/v1/sys/metrics` every 30 seconds.
- Avoid: `/v1/sys/metrics` is scraped every 30 seconds.

Passive voice is acceptable when the actor is genuinely unknown or irrelevant,
or when the object is the subject of the paragraph.

### 2.4 Use present tense

Describe what the system does, not what it will do or did.

- Good: OpenBao returns metrics in Prometheus exposition format.
- Avoid: OpenBao will return metrics in Prometheus exposition format.

Use past tense only for changelogs, incident write-ups, and historical context.
Use future tense only when the timing actually matters (for example, a
deprecation date).

### 2.5 Be direct; avoid hedging and filler

Cut these words unless they carry meaning:

- *simply*, *just*, *easily*, *quickly*, *obviously*, *of course*
- *please* in instructions
- *note that*, *it should be noted*, *as mentioned above*

Do not soften technical claims with *might*, *may*, *could potentially*, or
*sort of* unless the uncertainty is real. If it is real, state the condition.

- Good: If audit blocking is enabled, OpenBao stops servicing requests when no
  audit device can write.
- Avoid: OpenBao might potentially stop servicing requests in some cases.

### 2.6 Use *must*, *should*, and *can* deliberately

These words are load-bearing. Use them in the [RFC 2119][rfc-2119] sense.

| Word         | Meaning                                                                 |
| ------------ | ----------------------------------------------------------------------- |
| **must**     | Required. Skipping this breaks the design or the deployment.            |
| **must not** | Forbidden. Doing this breaks the design or the deployment.              |
| **should**   | Recommended. There are valid reasons to deviate; document them locally. |
| **can**      | Optional. Either choice is supported.                                   |

Do not use *should* when you mean *must*.

### 2.7 Use sentence case everywhere

Headings, table headers, list items, figure captions, and UI labels all use
sentence case. Capitalize proper nouns and product names only.

- Good: `### Configure the metrics listener`
- Avoid: `### Configure The Metrics Listener`

### 2.8 Punctuation and small mechanics

- Use the serial (Oxford) comma.
- Use American English spelling.
- Spell out *for example* and *that is*; do not use *e.g.* or *i.e.* in body
  text. They are acceptable inside parenthetical asides if space matters.
- Use one space after a period.
- Use straight quotes (`"`, `'`), not curly quotes.
- Do not use em dashes (`—`). Rewrite the sentence, use a comma, a colon, or
  parentheses, or split into two sentences.
- Use the en dash (`–`) for numeric ranges only.
- Place punctuation outside quotation marks when the punctuation is not part of
  the quoted material (logical style).

## 3. Document structure

### 3.1 Document types

Pick one type per page, and make the type obvious in the first paragraph.

| Type           | Purpose                                                    | Voice signal                                      |
| -------------- | ---------------------------------------------------------- | ------------------------------------------------- |
| **How-to**     | Accomplish a specific task end to end.                     | Numbered imperative steps; one outcome per page.  |
| **Runbook**    | Respond to a specific alert or operational condition.      | Imperative; preconditions, steps, verification.   |
| **Reference**  | Look up a fact: a contract, schema, label policy, metric.  | Tables and lists; minimal narrative.              |
| **Explainer**  | Explain why a design decision was made.                    | Prose; "you" still preferred over "we".           |

Do not mix types. If a page has both a how-to and an explainer in it, split
them and link.

### 3.2 Required front matter for every page

Every Markdown page in this repo starts with:

1. An H1 title in sentence case.
2. A one- or two-sentence summary paragraph that tells the reader what the page
   covers and who it is for.
3. (How-to and runbook only) A short *Before you begin* section listing
   preconditions, required permissions, and dependencies.

Do not use a separate "Introduction" heading. The summary paragraph is the
introduction.

### 3.3 Headings

- Use ATX-style headings (`#`, `##`, `###`).
- One H1 per page; it is the document title.
- Do not skip levels: an H4 must live under an H3.
- Keep headings short and scannable. Aim for under 60 characters.
- Headings describe the section content. Prefer noun phrases for reference
  sections and verb phrases for how-to sections.
  - Reference: `## Metric prefix strategy`
  - How-to: `## Configure the metrics listener`
- Do not end headings with punctuation.
- Do not put code, links, or emphasis in headings.

### 3.4 Section ordering

Within a how-to or runbook, sections appear in this order:

1. Summary paragraph (no heading).
2. *Before you begin*.
3. The procedure, broken into numbered steps under task-shaped subheadings.
4. *Verify the result*.
5. *Troubleshooting* (optional).
6. *What's next* (optional, with links to related pages).

Reference pages have no fixed ordering, but group related facts together and
put the most-queried facts first.

### 3.5 Paragraphs and lists

- Keep paragraphs under five lines. Split when you change subject.
- Use a bulleted list when order does not matter.
- Use a numbered list when order matters or when steps are referenced elsewhere
  ("see step 3").
- Use a table when you compare two or more things across the same attributes,
  or when the reader will scan rather than read.
- Do not nest lists more than two levels deep. If you need three, you need a
  table or a new subsection.

### 3.6 Tables

- Every column has a header in sentence case.
- Left-align text columns. Right-align numeric columns.
- Keep cells short. If a cell needs more than one sentence, move the content
  into a subsection and link to it.
- Do not put multi-line code blocks inside table cells. Use inline code only.
- Wide tables are acceptable; horizontal scroll is preferable to broken
  semantics.

### 3.7 Admonitions

Use admonitions sparingly. Reserve them for content the reader must not miss.
This repo uses GitHub-flavored alerts:

```markdown
> [!NOTE]
> Information that supplements the main content.

> [!WARNING]
> Action or condition that can cause data loss, outage, or security exposure.

> [!CAUTION]
> Irreversible or destructive action that needs explicit operator consent.
```

Do not invent new admonition types. Do not stack admonitions back to back; if
two warnings are required, write them into the prose.

### 3.8 Links

- Use descriptive link text. The link text alone should tell the reader where
  they are going.
  - Good: See the [OpenBao telemetry stanza][openbao-telemetry].
  - Avoid: See [here][openbao-telemetry] for telemetry config.
- Use reference-style links at the bottom of the file when a link appears more
  than once, or when the URL is long. Use inline links otherwise.
- Link to upstream documentation rather than restating it. If you restate
  something, link to the source on the same line.
- Internal links use relative paths from the current file
  (`../contracts/openbao-overview.md`). Do not use absolute paths or full
  GitHub URLs for content inside this repo.

### 3.9 Citations and footnotes

The reference architecture uses `[n]` footnote citations. Downstream
documentation does not. Use inline descriptive links in prose. If you need to
record provenance for a non-obvious claim, write a one-line *Source* note
beneath the relevant paragraph or table, with a link.

### 3.10 File and directory naming

- File names are lowercase, hyphen-separated, and end in `.md`.
- Names describe content, not document type, unless the type is part of the
  scope. Prefer `metrics-token.md` over `how-to-create-metrics-token.md`.
- Runbooks live under a `runbooks/` directory. Each runbook is named after the
  alert it responds to (`runbooks/openbao-sealed.md`).
- Contracts live under `contracts/`.
- Diagrams and images live next to the page that uses them, in an `assets/`
  subdirectory.

### 3.11 Line length

Soft-wrap prose at 80–100 characters. Do not hard-wrap inside fenced code
blocks. Do not break a sentence to hit a line length; readability wins.

## 4. Code, configuration, and command samples

Code samples are part of the documentation. Apply the same care to them as to
prose.

### 4.1 Language tags

Every fenced code block has a language tag. Use these tags consistently:

| Content                                        | Tag                                    |
| ---------------------------------------------- | -------------------------------------- |
| OpenBao configuration                          | `hcl`                                  |
| Kubernetes manifests, Helm values, Alloy river | `yaml` (or `river` for Alloy where supported) |
| Prometheus rules and scrape config             | `yaml`                                 |
| PromQL                                         | `promql`                               |
| LogQL                                          | `logql`                                |
| Shell commands                                 | `shell`                                |
| Plain output, ASCII diagrams                   | `text`                                 |
| JSON (audit samples, dashboard fragments)      | `json`                                 |

If a renderer does not understand a tag (for example `promql`), the syntax
still documents intent and survives future tooling.

### 4.2 Placeholders

Use angle brackets for values the reader must supply:

```hcl
listener "tcp" {
  address = "0.0.0.0:<metrics_port>"
}
```

Document every placeholder immediately under the snippet, in a bulleted list:

- `<metrics_port>`: TCP port for the private metrics listener. Default: `9101`.

Do not use `FIXME`, `TODO`, or `CHANGE_ME` as placeholders.

### 4.3 Completeness

Samples are one of two kinds. State which kind explicitly when it is not
obvious.

- **Runnable sample**: the snippet works when copied verbatim, with placeholders
  filled in. Required for how-tos and runbooks.
- **Sketch**: the snippet shows shape only and omits unrelated fields. Mark a
  sketch with a leading code comment such as `# sketch: omits TLS and logging`.

### 4.4 Comments inside samples

- Use comments to explain *why*, not *what*. Do not narrate syntax.
- Keep comments to one line where possible.
- Do not leave commented-out code in samples. Show the live config and explain
  alternatives in prose.

### 4.5 Shell commands

- One command per line. Do not chain with `&&` unless the chain is the point.
- Do not prefix prompts (`$`, `#`) inside `shell` blocks. The language tag
  already signals the type.
- Show only the output that the reader needs to verify the step. Trim the rest
  and replace it with `...` on its own line.

### 4.6 PromQL and LogQL

- One query per code block.
- Write the query on multiple lines when it improves readability:

```promql
max_over_time(
  vault_token_count[30m]
)
```

- For each non-trivial query, add a one-line plain-language description above
  the block explaining what question the query answers.
- Use the prefix variable `${p}` (matching the metric prefix strategy in the
  reference architecture) when the query must work for both `vault_*` and
  `openbao_*`. Document `${p}` once per page on first use.

### 4.7 Secrets and redaction

- Never include real tokens, certificates, private keys, IP addresses,
  hostnames, customer names, or audit payloads in samples.
- Use obvious placeholders: `s.REDACTED`, `<token>`, `example.internal`,
  `10.0.0.0/8`.
- Audit log examples must use the OpenBao docs' example payloads or
  hand-crafted fixtures. Never paste output from a live cluster.

## 5. Diagrams and figures

### 5.1 When to use which format

| Diagram need                                                     | Format                  |
| ---------------------------------------------------------------- | ----------------------- |
| Linear flow with a handful of boxes (signal pipeline, log path)  | ASCII inside `text` block |
| Sequence diagrams, state machines, decision trees, graphs        | Mermaid                 |
| Architecture overview where shape and density carry meaning      | SVG or PNG in `assets/` |
| Screenshots of Grafana, OpenBao UI, terminal output              | PNG in `assets/`        |

Prefer ASCII for simple flows; it diffs cleanly, renders everywhere, and
matches the existing reference architecture. Reach for Mermaid when ASCII would
need more than two columns or any non-linear edges.

### 5.2 ASCII diagrams

- Use a fenced `text` block.
- Use right arrows (`->`) for flow direction; do not mix `->` and `→` on the
  same page.
- Align boxes with spaces, not tabs.
- Keep each diagram under 80 characters wide so it renders without horizontal
  scroll.

### 5.3 Mermaid diagrams

- Use a fenced ```` ```mermaid ```` block.
- Label every node and edge. Unlabeled edges are not allowed.
- Keep node IDs short and lowercase; put the human-readable label in the node
  text (`api[OpenBao API]`).
- Pick one diagram type per figure. Do not mix flowcharts and sequence
  diagrams in a single block.

### 5.4 Images

- Store images under an `assets/` directory next to the page that uses them.
- File names are kebab-case and describe the content
  (`openbao-overview-dashboard.png`).
- Every image has descriptive alt text. The alt text must convey the
  information the image carries; do not use "screenshot" or "diagram" alone.
- Caption each image with italic text immediately below it, in sentence case,
  starting with *Figure N:*.

### 5.5 Figure numbering

Number figures within a page (`Figure 1`, `Figure 2`). Reference figures by
number in prose, not by phrases like *the figure above*.

## 6. Review checklist

Before opening a PR, run through this list. The same list is used during
review.

- [ ] The page has a clear document type (how-to, runbook, reference,
      explainer) and does not mix types.
- [ ] The first paragraph tells the reader who the page is for and what it
      covers.
- [ ] How-tos and runbooks have *Before you begin* and *Verify the result*
      sections.
- [ ] Voice is second-person and active; imperative for steps.
- [ ] No banned words (*simply*, *just*, *easily*, *please*, *note that*).
- [ ] *Must*, *should*, and *can* are used in the RFC 2119 sense.
- [ ] Headings are sentence case, descriptive, and do not skip levels.
- [ ] Every code block has a language tag.
- [ ] Every placeholder is documented under its snippet.
- [ ] Samples contain no real secrets, tokens, hostnames, or audit payloads.
- [ ] Every image has alt text and a numbered caption.
- [ ] Internal links are relative paths.
- [ ] Spelling, grammar, and link targets verified by linters or by reading.

## 7. Open style questions

This guide is the first version. The following are deliberately unresolved and
will be revisited:

| Question                                                        | Why it matters                                                |
| --------------------------------------------------------------- | ------------------------------------------------------------- |
| Terminology and naming conventions                              | Needs a separate dedicated guide; promised in section 1.      |
| Whether to lint prose with Vale (or similar) and a project config | Mechanical enforcement of *must/should/can*, banned words.   |
| Whether to standardize on a Markdown formatter (mdformat, prettier) | Avoid diff churn from tooling differences.                |
| How to mark version-specific behavior in reference pages          | OpenBao behavior changes across versions; need a convention. |

[google-style]: https://developers.google.com/style
[rfc-2119]: https://datatracker.ietf.org/doc/html/rfc2119
[openbao-telemetry]: https://openbao.org/docs/configuration/telemetry/
