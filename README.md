# ssmx

AWS SSM Session Manager CLI with configurable WebSocket keepalive.

## Why

AWS の `session-manager-plugin` は WebSocket の keepalive 間隔が **5分にハードコード** されています（[aws/session-manager-plugin#15](https://github.com/aws/session-manager-plugin/issues/15)）。

企業のプロキシやファイアウォールがこれより短い間隔でアイドル接続を切断するため、セッションが頻繁に切れる問題があります。`ssmx` は keepalive 間隔を自由に設定できるようにすることで、この問題を解決します。

## Features

- WebSocket keepalive 間隔のカスタマイズ（デフォルト: 15秒）
- fzf ライクなインタラクティブ選択（一覧は上、詳細は下に表示。`[EC2]` / `[ECS]` で種別を表示）
- ECS Exec によるコンテナへの接続
- `session-manager-plugin` 不要
- SSM Run Command によるインラインコマンド実行
- ローカルスクリプトを転送してリモート実行
- ローカルファイルを転送のみしてリモート配置（EC2 / ECS コンテナ）

## Install

### Homebrew

```bash
brew install inamuu/tap/ssmx
```

### Go

```bash
go install github.com/inamuu/ssmx@latest
```

### GitHub Releases

[Releases](https://github.com/inamuu/ssmx/releases) からバイナリをダウンロードしてください。

## Usage

```bash
# インタラクティブにインスタンスを選択して接続
ssmx

# プロファイル指定
ssmx -p production

# キープアライブ間隔を15秒に設定
ssmx -p production -k 15

# リージョン指定
ssmx -r ap-northeast-1

# インスタンスIDを直接指定（選択UIをスキップ）
ssmx -t i-0123456789abcdef0

# ECS コンテナを直接指定
ssmx -t ecs:my-cluster/0123456789abcdef/app

# 選択UIに ECS コンテナだけを表示（ec2 / ecs）
ssmx --type ecs

# SSM Run Command でコマンド実行
ssmx run --command 'uname -a'

# ローカルスクリプトを転送して実行
ssmx run --script ./scripts/deploy.sh -- --dry-run

# ローカルファイルを転送のみ
ssmx cp ./scripts/deploy.sh
ssmx cp ./scripts/deploy.sh /var/tmp/deploy.sh
```

## Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--profile` | `-p` | AWS プロファイル | `AWS_PROFILE` or `default` |
| `--keepalive` | `-k` | Keepalive 間隔（秒） | `15` |
| `--region` | `-r` | AWS リージョン | AWS config に従う |
| `--target` | `-t` | インスタンスID または `ecs:<cluster>/<task-id>/<container>` | - |
| `--type` | - | 選択UIに表示する種別を絞る（`ec2` / `ecs`） | 両方 |

## `run` サブコマンド

`ssmx run` は、SSM 管理下かつ Online な EC2 に対して AWS Run Command を実行します。
`--target` を省略すると、対象インスタンスをインタラクティブに選択します。

```bash
ssmx run --command 'hostname'
ssmx run --target i-0123456789abcdef0 --command 'ls -la /tmp/'
ssmx run --script ./scripts/deploy.sh -- --dry-run
```

`--command` にローカル実在ファイルを渡した場合は、`ssm-local-script` と同様にローカルスクリプトとして扱います。

## `cp` サブコマンド

`ssmx cp` は、SSM 管理下かつ Online な EC2、または ECS Exec が有効なコンテナにローカルファイルを転送します。転送後に実行はしません。
保存先を省略した場合は、既存の `run --script` と同じく `/tmp/<basename>` に配置します。

```bash
ssmx cp ./scripts/deploy.sh
ssmx cp ./scripts/deploy.sh /var/tmp/deploy.sh
ssmx cp --target i-0123456789abcdef0 ./config/app.env /etc/myapp/app.env

# ECS コンテナへ転送
ssmx cp --type ecs ./scripts/debug.sh
ssmx cp -t ecs:my-cluster/0123456789abcdef/app ./scripts/debug.sh /tmp/debug.sh
```

- EC2: AWS Run Command (`AWS-RunShellScript`) で配置します。
- ECS: ECS Exec のセッションで base64 エンコードしたファイルを流し込みます。コンテナに `/bin/sh` と `base64` が必要です。転送後にファイルサイズを照合します。

## リリース

main ブランチで変更をコミットし、作業ツリーがクリーンな状態で実行します（途中で `y/N` の確認があります）。

```bash
git add -A
git commit -m "..."

# テスト -> タグ作成・push -> GitHub Actions の完了待ち -> brew upgrade
scripts/release.sh patch      # minor / major / v0.2.0 も可

# ローカルでビルドして PATH 上の ssmx を置き換える（旧バイナリは ~/.Trash へ）
scripts/release.sh --local
```

## License

MIT
