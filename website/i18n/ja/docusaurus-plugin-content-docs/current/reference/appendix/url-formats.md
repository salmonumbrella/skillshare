---
sidebar_position: 4
---

# URL フォーマット

`skillshare install` が認識するすべての Source URL パターンです。

## クイックリファレンス

| フォーマット | 例 | 備考 |
|--------|---------|-------|
| GitHub の省略形 | `owner/repo` | `github.com/owner/repo` に展開される |
| サブディレクトリ付き GitHub | `owner/repo/path/to/skill` | リポジトリ内の特定の Skill をインストールする |
| 完全な HTTPS | `https://github.com/owner/repo` | 任意の Git ホスト |
| サブディレクトリ付き完全 HTTPS | `https://github.com/owner/repo/path` | host/owner/repo の後にサブディレクトリ |
| SSH | `git@github.com:owner/repo.git` | SSH キーによるプライベートリポジトリ |
| サブディレクトリ付き SSH | `git@github.com:owner/repo.git//path` | `//` でリポジトリとサブディレクトリを分ける |
| GHE Cloud | `mycompany.github.com/org/repo` | Enterprise Cloud のサブドメイン |
| GHE Server | `github.mycompany.com/org/repo` | Enterprise Server |
| Azure DevOps の省略形 | `ado:org/project/repo` | `dev.azure.com` の URL に展開される |
| Azure DevOps HTTPS | `https://dev.azure.com/org/proj/_git/repo` | 現代的なフォーマット |
| Azure DevOps SSH | `git@ssh.dev.azure.com:v3/org/proj/repo` | SSH v3 フォーマット |
| Azure DevOps Server | `https://custom-host/org/proj/_git/repo` | `azure_hosts` の設定が必要 |
| ローカルパス | `~/my-skill`、`/abs/path`、または `C:\path` | ディレクトリを Source にコピーする |
| Git file URL | `file:///path/to/repo` | ローカルの git clone（テスト用） |

## GitHub の省略形

最もシンプルなフォーマットです — 単に `owner/repo`:

```bash
skillshare install anthropics/skills
skillshare install ComposioHQ/awesome-claude-skills
```

これは内部的に `https://github.com/owner/repo` に展開されます。

### サブディレクトリ付き

`owner/repo` の後にパスを追加すると、特定の Skill をインストールできます。

```bash
skillshare install anthropics/skills/skills/pdf
skillshare install anthropics/skills/skills/commit
```

サブディレクトリが完全に一致しない場合、skillshare はそのベース名を持つ Skill をリポジトリ内で
スキャンします。

```bash
# "pdf" はルートには存在しないが、skills/pdf/ で見つかる — 自動的に解決される
skillshare install anthropics/skills/pdf
```

## 完全な HTTPS URL

任意の Git ホストで動作します。

```bash
# GitHub
skillshare install https://github.com/owner/repo

# GitLab
skillshare install https://gitlab.com/owner/repo

# Bitbucket
skillshare install https://bitbucket.org/owner/repo

# セルフホストの Gitea
skillshare install https://git.mycompany.com/team/skills

# AtomGit（中国）
skillshare install https://atomgit.com/owner/repo

# Gitee（中国）
skillshare install https://gitee.com/owner/repo
```

## SSH URL

プライベートリポジトリには SSH を使用します。

```bash
# 標準的な SSH
skillshare install git@github.com:owner/repo.git

# サブディレクトリ付き（// セパレータに注意）
skillshare install git@github.com:owner/repo.git//path/to/skill

# GitLab SSH
skillshare install git@gitlab.com:owner/repo.git
```

:::info `//` セパレータ
SSH URL では、`//` を使ってリポジトリとサブディレクトリのパスを分けます。これは SSH URL 内の `:` が
すでにセパレータとして機能しているため、標準的な `/` によるパス表記では曖昧になってしまうためです。
:::

## GitHub Enterprise

Enterprise のホスト名は自動的に認識されます。

```bash
# Enterprise Cloud（サブドメインパターン: *.github.com）
skillshare install mycompany.github.com/org/repo

# Enterprise Server（ホスト名パターン: github.*.*）
skillshare install github.mycompany.com/org/repo
skillshare install github.internal.corp/team/skills
```

どちらのパターンもサブディレクトリのパスに対応しています。

```bash
skillshare install github.mycompany.com/org/repo/path/to/skill
```

## Azure DevOps

### 省略形

`ado:` プレフィックスは Azure DevOps の URL に展開されます。

```bash
skillshare install ado:myorg/myproject/myrepo
skillshare install ado:myorg/myproject/myrepo/skills/react
```

### 完全な URL

```bash
# 現代的なフォーマット
skillshare install https://dev.azure.com/myorg/myproject/_git/myrepo

# レガシーなフォーマット（dev.azure.com に自動正規化される）
skillshare install https://myorg.visualstudio.com/myproject/_git/myrepo

# SSH
skillshare install git@ssh.dev.azure.com:v3/myorg/myproject/myrepo
```

## ローカルパス

自分のファイルシステム上のディレクトリからインストールします。

```bash
# 絶対パス
skillshare install /home/user/my-skill

# ホームディレクトリの省略形
skillshare install ~/my-skill

# 相対パス
skillshare install ./local-skill

# Windows のドライブレターパス
skillshare install D:\skills\my-skill
```

ローカルインストールはファイルを**コピー**します（シンボリックリンクではありません）。また
`skillshare update` では更新できません。

## 認証

### SSH キー（プライベートリポジトリに推奨）

```bash
# SSH キーが読み込まれていることを確認する
ssh-add ~/.ssh/id_ed25519

# SSH 経由でインストールする
skillshare install git@github.com:company/private-skills.git
```

### トークン付き HTTPS

HTTPS URL では、git は設定された credential helper を使用します。

```bash
# git credential helper を設定する（一度だけ）
git config --global credential.helper store

# または GitHub には GH CLI を使う
gh auth login

# それから通常通りインストールする
skillshare install https://github.com/company/private-repo
```

### PAT を使った Azure DevOps

Azure DevOps のリポジトリは HTTPS 認証に [Personal Access Tokens (PAT)](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops)
を使用します。

```bash
export AZURE_DEVOPS_TOKEN=your_pat_here
skillshare install https://dev.azure.com/org/project/_git/repo
```

または SSH を使う（トークン不要）:

```bash
skillshare install git@ssh.dev.azure.com:v3/org/project/repo
```

:::tip プライベートリポジトリ
HTTPS で認証エラーが出る場合は、SSH URL に切り替えてください。skillshare はハングする認証情報の
プロンプトを防ぐために `GIT_TERMINAL_PROMPT=0` を設定するため、インタラクティブな HTTPS 認証は
機能しません。
:::

## カスタム GitLab ドメイン {#custom-gitlab-domains}

名前に `gitlab` または `jihulab` を含むホスト（例: `gitlab.com`、`jihulab.com`、
`onprem.gitlab.internal`）は自動的に検出され、ネストされたサブグループに対応してパースされます。

カスタムドメイン上のセルフマネージドの GitLab インスタンス（例: `git.company.com`）については、
設定の [`gitlab_hosts`](../targets/configuration.md#gitlab_hosts) にホスト名を追加してください。

```yaml
gitlab_hosts:
  - git.company.com
```

これにより skillshare は、GitLab のネストされたサブグループの挙動に合わせて、URL パス全体を
リポジトリとして扱います。

**設定がない場合**、`.git` を使ってリポジトリパスの終端を示すことができます。

```bash
# git.company.com/team/frontend/ui（フルパスをリポジトリとして）からインストールする
skillshare install git.company.com/team/frontend/ui.git
```


## Gitea と CNB {#gitea-and-cnb}

`gitea.com`、名前に `gitea` を含む任意のホスト、および `cnb.cool` が認識されます。パスは
`owner/repo` として読み取られ、それ以降がサブディレクトリになります。

```bash
skillshare install https://gitea.com/owner/repo/skills/review
skillshare install https://cnb.cool/org/repo/skills
```

別のドメインでのセルフホストインスタンスについては、[`gitea_hosts`](../targets/configuration.md#gitea_hosts)
または [`cnb_hosts`](../targets/configuration.md#cnb_hosts) にホスト名を列挙してください。
プライベートリポジトリには [`GITEA_TOKEN`](./environment-variables.md#gitea_token) と
[`CNB_TOKEN`](./environment-variables.md#cnb_token) を使用します。

## カスタム Azure DevOps ドメイン {#custom-azure-domains}

組み込みの Azure DevOps パターンは、`dev.azure.com` と `*.visualstudio.com` に自動的に一致します。

カスタムドメイン上のセルフホストの Azure DevOps Server インスタンスについては、設定の
[`azure_hosts`](../targets/configuration.md#azure_hosts) にホスト名を追加してください。

```yaml
azure_hosts:
  - azuredevops.mycompany.com
```

これにより skillshare は、そのホスト上の `/_git/` を含む URL を Azure DevOps のパースにルーティング
するようになり、`.git` を追加することなく正しく clone URL を構築します。

## プラットフォーム対応状況

| 機能 | GitHub | GitLab | Bitbucket | Gitea | GHE | Azure DevOps | AtomGit/Gitee |
|---------|--------|--------|-----------|-------|-----|--------------|---------------|
| 省略形（`owner/repo`） | あり | なし | なし | なし | あり | `ado:` プレフィックス | なし |
| 完全な HTTPS URL | あり | あり | あり | あり | あり | あり | あり |
| SSH URL | あり | あり | あり | あり | あり | あり | あり |
| サブディレクトリ | あり | あり | あり | あり | あり | あり | あり |
| `skillshare search` | あり | なし | なし | なし | なし | なし | なし |

## 関連項目

- [Install コマンド](/docs/reference/commands/install) — 完全なインストールオプションと例
