# ssmx

AWS SSM Session Manager CLI with configurable WebSocket keepalive.

## Why

AWS の `session-manager-plugin` は WebSocket の keepalive 間隔が **5分にハードコード** されています（[aws/session-manager-plugin#15](https://github.com/aws/session-manager-plugin/issues/15)）。

企業のプロキシやファイアウォールがこれより短い間隔でアイドル接続を切断するため、セッションが頻繁に切れる問題があります。`ssmx` は keepalive 間隔を自由に設定できるようにすることで、この問題を解決します。

## Features

- WebSocket keepalive 間隔のカスタマイズ（デフォルト: 15秒）
- fzf ライクなインタラクティブインスタンス選択
- `session-manager-plugin` 不要

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
```

## Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--profile` | `-p` | AWS プロファイル | `AWS_PROFILE` or `default` |
| `--keepalive` | `-k` | Keepalive 間隔（秒） | `15` |
| `--region` | `-r` | AWS リージョン | AWS config に従う |
| `--target` | `-t` | インスタンスID（直接指定） | - |

## License

MIT
