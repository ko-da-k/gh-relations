# gh-relations

A [`gh extension`](https://cli.github.com) that draws linked Issues and pull requests as a graph in
the terminal

## Example

```console
$ gh relations https://github.com/ko-da-k/gh-relations/pull/2
◇ #2  【for Test】closes #3, mentions #5 and #7
├▸ ◯ #3  【for Test】closed by #2, sub of #4, blocked by #7
│  ├▴ ◯ #4  【for Test】has 3 sub-issues: #3, #5, fixtures#8
│  │  ├▾ ◯ #5  【for Test】sub of #4, closed by #9
│  │  │  ├▸ ◇ #9  【for Test】closes #5  +
│  │  │  └@ ◇ #2  【for Test】closes #3, mentions #5 and #7  ↱
│  │  ├▾ ◯ gh-relations-fixtures#8  【for Test】sub of gh-relations#4
│  │  └@ ◇ #8  【for Test】duplicate of #11 by comment, mentions #4
│  │     └= ◯ #11  【for Test】#8 is marked a duplicate of this by comment  +
│  ├⊘ ◯ #7  【for Test】blocks #3, blocked by fixtures#7, closed by fixtures#9
│  │  ├▸ ◇ gh-relations-fixtures#9  【for Test】closes gh-relations#7
│  │  ├⊘ ◯ gh-relations-fixtures#7  【for Test】blocks gh-relations#7, mentions gh-relations#2  ↱
│  │  └@ ◇ #2  【for Test】closes #3, mentions #5 and #7  ↱
│  └@ ◯ #6  【for Test】tracked in #10, fixtures#10 and fixtures#11, mentions #3
│     ├▪ ◯ #10  【for Test】tracks #6, relates to fixtures#11
│     │  └~ ◯ gh-relations-fixtures#11  【for Test】tracks gh-relations#6, relates to gh-relations#10  ↱
│     ├▪ ◯ gh-relations-fixtures#10  【for Test】tracks gh-relations#6
│     └▪ ◯ gh-relations-fixtures#11  【for Test】tracks gh-relations#6, relates to gh-relations#10
└@ ◯ gh-relations-fixtures#7  【for Test】blocks gh-relations#7, mentions gh-relations#2

kind   ◯ issue  ◇ pull request
state  open  merged  closed  draft  not planned
link   what this line is to its parent above:
       ▸ closes/closed by  ▴ parent of  ▾ sub of  ⊘ blocks  ~ relates to  = duplicate  ▪ tracks  @ mentions
mark   + not expanded  ↱ drawn above
```

The tree above is real output. Every way GitHub can link an Issue or a pull request is read through
its API and drawn. References such as `#2` and `gh-relations-fixtures#8` are hyperlinks, so on a Mac
cmd+click opens them in a browser.

## Install

```bash
gh extension install ko-da-k/gh-relations
```

## Usage

```bash
gh relations [<number> | <url>] [flags]
```

| flag | default | description |
|---|---|---|
| `--repo OWNER/REPO` | current repository | where a bare number is looked up |
| `--depth N` | `4` | how many levels of the graph to draw. Every edge counts as one level |
| `--scope repo\|org\|all` | `org` | where expansion may happen. A node outside the scope is still drawn, just not expanded |
| `--max-nodes N` | `50` | stop once this many nodes are in the graph |
| `--no-color` | off | print the state as text instead of colour |

## Legend

| | |
|---|---|
| `◯` / `◇` | Issue / pull request |
| green / purple / red / grey | open / merged / closed / draft or closed as not planned |
| `▸` | close or closed by |
| `▴` / `▾` | parent / sub-issue |
| `⊘` / `⊚` | blocking / blocked by |
| `~` | related |
| `=` | duplicate |
| `▪` / `▫` | tracking / tracked by |
| `@` | mentioned |
| `↱` | an Issue/PR drawn elsewhere in the graph already |
| `+` | not expanded, because a limit stopped it |
