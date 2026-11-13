---
sidebar_position: 3
---

# レシピ: Pre-commit フック

> [pre-commit](https://pre-commit.com/) フレームワークを使い、すべてのコミットで `skillshare audit`
> を自動実行する。

## いつ使うか

Pre-commit フックが特に役立つのは次のような場合です。

- **複数のコントリビューターが Skill を編集する** — チームメンバーが誤って危険なコマンド
  （`curl | bash`、`sudo rm -rf`）を混入させることがあります。フックはこれらがバージョン管理に
  入る前に捕捉します。
- **Skill が外部ソースから来る** — GitHub、コミュニティリポジトリ、AI 生成コンテンツから Skill を
  コピーすると手動レビューが困難になります。自動スキャンが安全網になります。
- **即座のフィードバックが欲しい** — CI も問題を捕捉しますが、それは push した後のみです。フックは
  開発者に数秒でローカルの即座のフィードバックを与えます。

以下の場合はスキップできます。

- あなたが唯一の作者であり、すべての Skill を信頼している
- Skill がほとんど変更されない（フックは `.skillshare/` または `skills/` のファイルが変更された
  ときのみ実行される）

## セットアップ

プロジェクトの `.pre-commit-config.yaml` に追加します。

```yaml
repos:
  - repo: https://github.com/runkids/skillshare
    rev: v0.16.8  # use latest release tag
    hooks:
      - id: skillshare-audit
```

その後、フックをインストールします。

```bash
pre-commit install
```

## 仕組み

このフックは、`.skillshare/` または `skills/` ディレクトリに一致するファイルへの変更をコミットする
たびに `skillshare audit -p` を実行します。設定されたしきい値を超える検出結果があれば、コミットは
ブロックされます。

## 設定

このフックはプロジェクトの `.skillshare/config.yaml` の設定に従います。

```yaml
audit:
  block_threshold: high  # block on HIGH+ findings
```

## フックをスキップする

一度だけスキップするには:

```bash
SKIP=skillshare-audit git commit -m "your message"
```

## 要件

- `skillshare` CLI がインストールされ `PATH` から利用できること
- プロジェクトが `skillshare init -p` で初期化されていること

## CI との組み合わせ

Pre-commit フックはローカルで問題を捕捉し、[CI/CD 検証](ci-cd-skill-validation.md) はチーム全体の
安全網を提供します。多層防御のために両方を使ってください。
