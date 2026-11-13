---
sidebar_position: 1
---

# 为什么选择本地优先

> skillshare 是一个没有运行时依赖的单一二进制文件。原因如下。

## 这个决定

skillshare 以单一 Go 二进制文件的形式发布。不需要 Node.js，不需要 Python，不需要包管理器，不需要守护进程。安装、运行，完事。

这并非阻力最小的路径——而是出于三条原则的刻意选择。

## 原则一：零依赖链

每一个依赖项都是一个攻击面，也是一份维护负担。

如果 skillshare 需要 Node.js，你就得管理 Node 版本、处理 `node_modules`、应对平台专属的原生模块，还要信任整条 npm 供应链。对于一个用来管理 AI Skills——而 Skills 本身就是不可信内容——的工具来说，再引入一条不可信的依赖链是不可接受的。

Go 会编译成静态二进制文件。依赖链在编译时就结束了。你下载的是什么，运行的就是什么。

## 原则二：在任何地方都以同样的方式工作

skillshare 可以运行在：
- macOS（Intel 与 Apple Silicon）
- Linux（amd64 与 arm64）
- Windows（amd64）
- Docker 容器（无需特殊设置）
- CI/CD 流水线（无需语言运行时）
- Dev container 与 Codespaces

单一二进制文件意味着在所有平台上行为完全一致。不会再有“在我机器上明明能跑”的调试难题，也不会有 CI 环境的偏差。

## 原则三：默认离线

skillshare 的核心操作——`sync`、`list`、`status`、`backup`、`restore`——都不需要联网即可工作。只有那些明确需要访问远程的操作（`install`、`search`、`check`、`update`、`push`、`pull`）才需要联网。

这一点在以下场景中尤为重要：
- **物理隔离（air-gapped）环境**：国防、医疗、金融机构通常会限制网络访问
- **网络不稳定的场合**：火车上、飞机上、会议现场的 WiFi
- **速度**：本地操作能在毫秒级完成，而不是秒级

## 为什么不做成包管理器插件？

我们曾考虑过以 npm 包、Homebrew formula（现已作为额外的分发渠道支持）或 pip 包的形式发布。但每一种都有同样的问题：它们都会引入一个 skillshare 用户可能并不具备、也不想要的运行时依赖。

一个在 Windows 上使用 Cursor 的开发者不应该被要求安装 Homebrew。一条运行 Alpine Linux 的 CI 流水线也不应该被要求安装 Node.js。工具应该去适应用户的环境，而不是反过来。

## 权衡取舍

单一二进制文件的方案是有代价的：

- **构建复杂度**：需要为 6 个以上的目标平台做交叉编译，并禁用 CGO
- **更新机制**：没有 `npm update` 这样的命令——skillshare 有自己的 `upgrade` 命令
- **UI 交付**：Web 控制台无法与二进制文件打包在一起（体积太大），因此会在运行时下载并缓存

我们接受这些代价，因为它们换来的是简单的用户体验：下载、运行，完事。

## 相关内容

- [安全优先的设计](/docs/understand/philosophy/security-first)
- [与其他工具的比较](/docs/understand/philosophy/comparison)
