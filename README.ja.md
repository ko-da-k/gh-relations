# gh-relations

[English](README.md) | [日本語](README.ja.md)

リンクでつながった Issue と pull request を、ターミナルでグラフ形式で表示する [`gh extension`](https://cli.github.com)

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

上記は実際の描画サンプルです。
GitHub 上で Issue/PR の紐づけが可能なものを API で取得し描画しています。
`#2`, `gh-relations-fixtures#8` などは Link となっているため、Mac では cmd+click でブラウザで開くことができます。

## インストール

```bash
gh extension install ko-da-k/gh-relations
```

## 使い方

```bash
gh relations [<number> | <url>] [flags]
```

| flag | default | description |
|---|---|---|
| `--repo OWNER/REPO` | current repository | 裸の番号をどのリポジトリで引くか |
| `--depth N` | `4` | graph を何段まで描画するか。どの種類の辺も 1 段として数える |
| `--scope repo\|org\|all` | `org` | 展開してよい範囲。範囲外のノードも描かれるが、展開はされない |
| `--max-nodes N` | `50` | グラフのノードがこの数に達したら打ち切る |
| `--no-color` | off | 状態を色ではなく文字で書く |

## 凡例

| | |
|---|---|
| `◯` / `◇` | Issue / pull request |
| 緑・紫・赤・灰 | open・merged・closed・draft または not planned として closed |
| `▸` | close or closed by |
| `▴` / `▾` | parent / sub-issue |
| `⊘` / `⊚` | blocking / blocked by |
| `~` | related |
| `=` | duplicate |
| `▪` / `▫` | tracking / tracked by |
| `@` | mentioned |
| `↱` | グラフで重複して書かれている Issue/PR |
| `+` | 制限によって展開されなかった |
