# ssmx

AWS SSM Session Manager CLI with configurable WebSocket keepalive.

## Why

AWS の `session-manager-plugin` は WebSocket の keepalive 間隔が **5分にハードコード** されています（[aws/session-manager-plugin#15](https://github.com/aws/session-manager-plugin/issues/15)）。

企業のプロキシやファイアウォールがこれより短い間隔でアイドル接続を切断するため、セッションが頻繁に切れる問題があります。`ssmx` は keepalive 間隔を自由に設定できるようにすることで、この問題を解決します。

## Features

- WebSocket keepalive 間隔のカスタマイズ（デフォルト: 15秒）
- fzf ライクなインタラクティブインスタンス選択
- `session-manager-plugin` 不要
- SSM Run Command によるインラインコマンド実行
- ローカルスクリプトを転送してリモート実行

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

# SSM Run Command でコマンド実行
ssmx run --command 'uname -a'

# ローカルスクリプトを転送して実行
ssmx run --script ./scripts/deploy.sh -- --dry-run
```

## Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--profile` | `-p` | AWS プロファイル | `AWS_PROFILE` or `default` |
| `--keepalive` | `-k` | Keepalive 間隔（秒） | `15` |
| `--region` | `-r` | AWS リージョン | AWS config に従う |
| `--target` | `-t` | インスタンスID（直接指定） | - |

## `run` サブコマンド

`ssmx run` は、SSM 管理下かつ Online な EC2 に対して AWS Run Command を実行します。
`--target` を省略すると、対象インスタンスをインタラクティブに選択します。

```bash
ssmx run --command 'hostname'
ssmx run --target i-0123456789abcdef0 --command 'ls -la /tmp/'
ssmx run --script ./scripts/deploy.sh -- --dry-run
```

`--command` にローカル実在ファイルを渡した場合は、`ssm-local-script` と同様にローカルスクリプトとして扱います。

## License

MIT
