---
sidebar_position: 4
---

# URL 格式

`skillshare install` 所辨識的所有 source URL 模式。

## 快速參考

| Format | Example | Notes |
|--------|---------|-------|
| GitHub shorthand | `owner/repo` | 展開為 `github.com/owner/repo` |
| GitHub with subdir | `owner/repo/path/to/skill` | 從 repo 安裝特定的 skill |
| Full HTTPS | `https://github.com/owner/repo` | 支援任何 Git host |
| Full HTTPS with subdir | `https://github.com/owner/repo/path` | 子目錄接在 host/owner/repo 之後 |
| SSH | `git@github.com:owner/repo.git` | 透過 SSH 金鑰存取私有 repo |
| SSH with subdir | `git@github.com:owner/repo.git//path` | 用 `//` 將 repo 與子目錄分隔開 |
| GHE Cloud | `mycompany.github.com/org/repo` | Enterprise Cloud 子網域 |
| GHE Server | `github.mycompany.com/org/repo` | Enterprise Server |
| Azure DevOps shorthand | `ado:org/project/repo` | 展開為 `dev.azure.com` URL |
| Azure DevOps HTTPS | `https://dev.azure.com/org/proj/_git/repo` | 現代格式 |
| Azure DevOps SSH | `git@ssh.dev.azure.com:v3/org/proj/repo` | SSH v3 格式 |
| Azure DevOps Server | `https://custom-host/org/proj/_git/repo` | 需要 `azure_hosts` 設定 |
| Local path | `~/my-skill`、`/abs/path` 或 `C:\path` | 將目錄複製到 source |
| Git file URL | `file:///path/to/repo` | 本機 git clone（用於測試） |

## GitHub Shorthand

最簡單的格式 — 只需要 `owner/repo`：

```bash
skillshare install anthropics/skills
skillshare install ComposioHQ/awesome-claude-skills
```

這會在內部展開為 `https://github.com/owner/repo`。

### 搭配子目錄

在 `owner/repo` 之後加上路徑，即可安裝特定的 skill：

```bash
skillshare install anthropics/skills/skills/pdf
skillshare install anthropics/skills/skills/commit
```

當子目錄無法完全對應時，skillshare 會掃描該 repo，尋找符合該基本名稱的 skill：

```bash
# 根目錄沒有 "pdf"，但在 skills/pdf/ 找到 — 會自動解析
skillshare install anthropics/skills/pdf
```

## Full HTTPS URLs

適用於任何 Git host：

```bash
# GitHub
skillshare install https://github.com/owner/repo

# GitLab
skillshare install https://gitlab.com/owner/repo

# Bitbucket
skillshare install https://bitbucket.org/owner/repo

# 自架 Gitea
skillshare install https://git.mycompany.com/team/skills

# AtomGit（中國）
skillshare install https://atomgit.com/owner/repo

# Gitee（中國）
skillshare install https://gitee.com/owner/repo
```

## SSH URLs

私有儲存庫請使用 SSH：

```bash
# 標準 SSH
skillshare install git@github.com:owner/repo.git

# 搭配子目錄（注意 // 分隔符）
skillshare install git@github.com:owner/repo.git//path/to/skill

# GitLab SSH
skillshare install git@gitlab.com:owner/repo.git
```

:::info `//` 分隔符
對於 SSH URL，請用 `//` 將 repo 與子目錄路徑分隔開。這是因為 SSH URL 中的 `:` 已經作為分隔符使用，因此標準的 `/` 路徑慣例會造成歧義。
:::

## GitHub Enterprise

Enterprise 主機名稱會被自動辨識：

```bash
# Enterprise Cloud（子網域模式：*.github.com）
skillshare install mycompany.github.com/org/repo

# Enterprise Server（主機名稱模式：github.*.*）
skillshare install github.mycompany.com/org/repo
skillshare install github.internal.corp/team/skills
```

兩種模式都支援子目錄路徑：

```bash
skillshare install github.mycompany.com/org/repo/path/to/skill
```

## Azure DevOps

### Shorthand

`ado:` 前綴會展開為 Azure DevOps URL：

```bash
skillshare install ado:myorg/myproject/myrepo
skillshare install ado:myorg/myproject/myrepo/skills/react
```

### Full URLs

```bash
# 現代格式
skillshare install https://dev.azure.com/myorg/myproject/_git/myrepo

# 舊格式（會自動正規化為 dev.azure.com）
skillshare install https://myorg.visualstudio.com/myproject/_git/myrepo

# SSH
skillshare install git@ssh.dev.azure.com:v3/myorg/myproject/myrepo
```

## Local Paths

從你檔案系統上的某個目錄安裝：

```bash
# 絕對路徑
skillshare install /home/user/my-skill

# 家目錄簡寫
skillshare install ~/my-skill

# 相對路徑
skillshare install ./local-skill

# Windows 磁碟機代號路徑
skillshare install D:\skills\my-skill
```

Local 安裝會**複製**檔案（而非 symlink），且無法透過 `skillshare update` 更新。

## 驗證

### SSH 金鑰（建議用於私有 Repo）

```bash
# 確認你的 SSH 金鑰已載入
ssh-add ~/.ssh/id_ed25519

# 透過 SSH 安裝
skillshare install git@github.com:company/private-skills.git
```

### 搭配 Token 的 HTTPS

對於 HTTPS URL，git 會使用你設定好的 credential helper：

```bash
# 設定 git credential helper（一次性）
git config --global credential.helper store

# 或針對 GitHub 使用 GH CLI
gh auth login

# 接著照常安裝
skillshare install https://github.com/company/private-repo
```

### 搭配 PAT 的 Azure DevOps

Azure DevOps 的 repo 使用 [Personal Access Tokens（PATs）](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops) 進行 HTTPS 驗證：

```bash
export AZURE_DEVOPS_TOKEN=your_pat_here
skillshare install https://dev.azure.com/org/project/_git/repo
```

或改用 SSH（不需要 token）：

```bash
skillshare install git@ssh.dev.azure.com:v3/org/project/repo
```

:::tip 私有 Repo
如果使用 HTTPS 時發生驗證錯誤，請改用 SSH URL。skillshare 會設定 `GIT_TERMINAL_PROMPT=0` 以避免憑證提示卡住，因此互動式的 HTTPS 驗證不會生效。
:::

## 自訂 GitLab 網域 {#custom-gitlab-domains}

名稱中含有 `gitlab` 或 `jihulab` 的主機（例如 `gitlab.com`、`jihulab.com`、`onprem.gitlab.internal`）會被自動偵測，並以支援巢狀 subgroup 的方式解析。

對於使用自訂網域的自管 GitLab 執行個體（例如 `git.company.com`），請將該主機名稱加入你設定檔中的 [`gitlab_hosts`](../targets/configuration.md#gitlab_hosts)：

```yaml
gitlab_hosts:
  - git.company.com
```

這會告訴 skillshare 將完整的 URL 路徑當作儲存庫，與 GitLab 的巢狀 subgroup 行為一致。

**若沒有設定**，你可以用 `.git` 標示 repo 路徑的結尾：

```bash
# 從 git.company.com/team/frontend/ui 安裝（將完整路徑當作 repo）
skillshare install git.company.com/team/frontend/ui.git
```


## Gitea 與 CNB {#gitea-and-cnb}

`gitea.com`、任何名稱含有 `gitea` 的主機，以及 `cnb.cool` 都會被辨識。路徑會被解讀為 `owner/repo`，之後的部分則是子目錄：

```bash
skillshare install https://gitea.com/owner/repo/skills/review
skillshare install https://cnb.cool/org/repo/skills
```

對於使用其他網域的自架執行個體，請將主機名稱列在 [`gitea_hosts`](../targets/configuration.md#gitea_hosts) 或 [`cnb_hosts`](../targets/configuration.md#cnb_hosts) 中。私有 repo 分別使用 [`GITEA_TOKEN`](./environment-variables.md#gitea_token) 與 [`CNB_TOKEN`](./environment-variables.md#cnb_token)。

## 自訂 Azure DevOps 網域 {#custom-azure-domains}

內建的 Azure DevOps 模式會自動比對 `dev.azure.com` 與 `*.visualstudio.com`。

對於使用自訂網域的自架 Azure DevOps Server 執行個體，請將該主機名稱加入你設定檔中的 [`azure_hosts`](../targets/configuration.md#azure_hosts)：

```yaml
azure_hosts:
  - azuredevops.mycompany.com
```

這會告訴 skillshare 將該主機上帶有 `/_git/` 的 URL 透過 Azure DevOps 解析路徑處理，正確建構 clone URL，而不會額外附加 `.git`。

## 平台支援

| Feature | GitHub | GitLab | Bitbucket | Gitea | GHE | Azure DevOps | AtomGit/Gitee |
|---------|--------|--------|-----------|-------|-----|--------------|---------------|
| Shorthand (`owner/repo`) | Yes | No | No | No | Yes | `ado:` prefix | No |
| Full HTTPS URL | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| SSH URL | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Subdirectory | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `skillshare search` | Yes | No | No | No | No | No | No |

## 相關文件

- [Install command](/docs/reference/commands/install) — 完整的安裝選項與範例
