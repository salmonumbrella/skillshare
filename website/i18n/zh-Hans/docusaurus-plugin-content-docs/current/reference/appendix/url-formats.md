---
sidebar_position: 4
---

# URL Formats

`skillshare install` 识别的所有来源 URL 格式。

## Quick Reference

| Format | Example | Notes |
|--------|---------|-------|
| GitHub shorthand | `owner/repo` | 展开为 `github.com/owner/repo` |
| GitHub with subdir | `owner/repo/path/to/skill` | 从仓库中安装指定的 skill |
| Full HTTPS | `https://github.com/owner/repo` | 适用于任意 Git host |
| Full HTTPS with subdir | `https://github.com/owner/repo/path` | 子目录接在 host/owner/repo 之后 |
| SSH | `git@github.com:owner/repo.git` | 通过 SSH key 安装私有仓库 |
| SSH with subdir | `git@github.com:owner/repo.git//path` | 用 `//` 分隔仓库与子目录 |
| GHE Cloud | `mycompany.github.com/org/repo` | Enterprise Cloud 子域名 |
| GHE Server | `github.mycompany.com/org/repo` | Enterprise Server |
| Azure DevOps shorthand | `ado:org/project/repo` | 展开为 `dev.azure.com` URL |
| Azure DevOps HTTPS | `https://dev.azure.com/org/proj/_git/repo` | 现代格式 |
| Azure DevOps SSH | `git@ssh.dev.azure.com:v3/org/proj/repo` | SSH v3 格式 |
| Azure DevOps Server | `https://custom-host/org/proj/_git/repo` | 需要 `azure_hosts` 配置 |
| Local path | `~/my-skill`、`/abs/path` 或 `C:\path` | 将目录复制到 source |
| Git file URL | `file:///path/to/repo` | 本地 git clone（用于测试） |

## GitHub Shorthand

最简单的格式 — 直接写 `owner/repo`：

```bash
skillshare install anthropics/skills
skillshare install ComposioHQ/awesome-claude-skills
```

内部会展开为 `https://github.com/owner/repo`。

### With Subdirectory

在 `owner/repo` 后加一个路径，安装指定的 skill：

```bash
skillshare install anthropics/skills/skills/pdf
skillshare install anthropics/skills/skills/commit
```

当子目录不完全匹配时，skillshare 会在仓库中扫描与该 basename 同名的 skill：

```bash
# 根目录下没有 "pdf"，但在 skills/pdf/ 中找到 — 自动解析
skillshare install anthropics/skills/pdf
```

## Full HTTPS URLs

适用于任意 Git host：

```bash
# GitHub
skillshare install https://github.com/owner/repo

# GitLab
skillshare install https://gitlab.com/owner/repo

# Bitbucket
skillshare install https://bitbucket.org/owner/repo

# 自建 Gitea
skillshare install https://git.mycompany.com/team/skills

# AtomGit（中国）
skillshare install https://atomgit.com/owner/repo

# Gitee（中国）
skillshare install https://gitee.com/owner/repo
```

## SSH URLs

对私有仓库使用 SSH：

```bash
# 标准 SSH
skillshare install git@github.com:owner/repo.git

# 带子目录（注意 // 分隔符）
skillshare install git@github.com:owner/repo.git//path/to/skill

# GitLab SSH
skillshare install git@gitlab.com:owner/repo.git
```

:::info The `//` separator
对于 SSH URL，用 `//` 分隔仓库与子目录路径。这是因为 SSH URL 中的 `:` 已经充当了分隔符，标准的 `/` 路径写法会产生歧义。
:::

## GitHub Enterprise

Enterprise 主机名会被自动识别：

```bash
# Enterprise Cloud（子域名模式：*.github.com）
skillshare install mycompany.github.com/org/repo

# Enterprise Server（主机名模式：github.*.*）
skillshare install github.mycompany.com/org/repo
skillshare install github.internal.corp/team/skills
```

两种模式都支持子目录路径：

```bash
skillshare install github.mycompany.com/org/repo/path/to/skill
```

## Azure DevOps

### Shorthand

`ado:` 前缀会展开为 Azure DevOps URL：

```bash
skillshare install ado:myorg/myproject/myrepo
skillshare install ado:myorg/myproject/myrepo/skills/react
```

### Full URLs

```bash
# 现代格式
skillshare install https://dev.azure.com/myorg/myproject/_git/myrepo

# 旧版格式（自动规范化为 dev.azure.com）
skillshare install https://myorg.visualstudio.com/myproject/_git/myrepo

# SSH
skillshare install git@ssh.dev.azure.com:v3/myorg/myproject/myrepo
```

## Local Paths

从本地文件系统的目录安装：

```bash
# 绝对路径
skillshare install /home/user/my-skill

# 主目录简写
skillshare install ~/my-skill

# 相对路径
skillshare install ./local-skill

# Windows 盘符路径
skillshare install D:\skills\my-skill
```

本地安装是**复制**文件（而非符号链接），且无法通过 `skillshare update` 更新。

## Authentication

### SSH Keys (Recommended for Private Repos)

```bash
# 确保你的 SSH key 已加载
ssh-add ~/.ssh/id_ed25519

# 通过 SSH 安装
skillshare install git@github.com:company/private-skills.git
```

### HTTPS with Tokens

对于 HTTPS URL，git 会使用你配置的凭证助手（credential helper）：

```bash
# 配置 git 凭证助手（一次性）
git config --global credential.helper store

# 或使用 GH CLI 登录 GitHub
gh auth login

# 然后正常安装
skillshare install https://github.com/company/private-repo
```

### Azure DevOps with PAT

Azure DevOps 仓库通过 [Personal Access Tokens (PATs)](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops) 进行 HTTPS 认证：

```bash
export AZURE_DEVOPS_TOKEN=your_pat_here
skillshare install https://dev.azure.com/org/project/_git/repo
```

或使用 SSH（无需令牌）：

```bash
skillshare install git@ssh.dev.azure.com:v3/org/project/repo
```

:::tip Private Repos
如果 HTTPS 出现认证错误，请改用 SSH URL。skillshare 会设置 `GIT_TERMINAL_PROMPT=0` 以防止凭证提示挂起，因此交互式 HTTPS 认证无法使用。
:::

## Custom GitLab Domains {#custom-gitlab-domains}

名称中包含 `gitlab` 或 `jihulab` 的主机（例如 `gitlab.com`、`jihulab.com`、`onprem.gitlab.internal`）会被自动识别，并按嵌套子群组（nested subgroup）支持进行解析。

对于自建在自定义域名上的 GitLab 实例（例如 `git.company.com`），需要在配置文件的 [`gitlab_hosts`](../targets/configuration.md#gitlab_hosts) 中添加该主机名：

```yaml
gitlab_hosts:
  - git.company.com
```

这会告知 skillshare 将完整的 URL 路径作为仓库处理，以匹配 GitLab 的嵌套子群组行为。

**没有配置时**，可以用 `.git` 标记仓库路径的结尾：

```bash
# 从 git.company.com/team/frontend/ui 安装（完整路径作为仓库）
skillshare install git.company.com/team/frontend/ui.git
```


## Gitea and CNB {#gitea-and-cnb}

`gitea.com`、名称中包含 `gitea` 的任意主机，以及 `cnb.cool` 都会被识别。路径会被解析为 `owner/repo`，其后的部分则作为子目录：

```bash
skillshare install https://gitea.com/owner/repo/skills/review
skillshare install https://cnb.cool/org/repo/skills
```

对于部署在其他域名上的自建实例，请在 [`gitea_hosts`](../targets/configuration.md#gitea_hosts) 或 [`cnb_hosts`](../targets/configuration.md#cnb_hosts) 中列出该主机名。私有仓库需使用 [`GITEA_TOKEN`](./environment-variables.md#gitea_token) 和 [`CNB_TOKEN`](./environment-variables.md#cnb_token)。

## Custom Azure DevOps Domains {#custom-azure-domains}

内置的 Azure DevOps 模式会自动匹配 `dev.azure.com` 与 `*.visualstudio.com`。

对于自建在自定义域名上的 Azure DevOps Server 实例，需要在配置文件的 [`azure_hosts`](../targets/configuration.md#azure_hosts) 中添加该主机名：

```yaml
azure_hosts:
  - azuredevops.mycompany.com
```

这会告知 skillshare 将该主机上带有 `/_git/` 的 URL 通过 Azure DevOps 解析逻辑处理，从而正确构造 clone URL 而不会附加 `.git`。

## Platform Support

| Feature | GitHub | GitLab | Bitbucket | Gitea | GHE | Azure DevOps | AtomGit/Gitee |
|---------|--------|--------|-----------|-------|-----|--------------|---------------|
| Shorthand (`owner/repo`) | 支持 | 不支持 | 不支持 | 不支持 | 支持 | `ado:` 前缀 | 不支持 |
| Full HTTPS URL | 支持 | 支持 | 支持 | 支持 | 支持 | 支持 | 支持 |
| SSH URL | 支持 | 支持 | 支持 | 支持 | 支持 | 支持 | 支持 |
| Subdirectory | 支持 | 支持 | 支持 | 支持 | 支持 | 支持 | 支持 |
| `skillshare search` | 支持 | 不支持 | 不支持 | 不支持 | 不支持 | 不支持 | 不支持 |

## Related

- [Install command](/docs/reference/commands/install) — 完整的 install 选项与示例
