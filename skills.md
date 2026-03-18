# ssmx - SSM Session Manager CLI with Keepalive Support

## 背景・動機

AWS SSM Session Manager の `session-manager-plugin` は WebSocket の keepalive 間隔（`PingTimeInterval`）が **5分にハードコード** されている（[GitHub Issue #15](https://github.com/aws/session-manager-plugin/issues/15)）。

企業のプロキシやファイアウォールがこれより短い間隔（1分程度）でアイドルなWebSocket接続を切断するため、セッションが頻繁に切れる問題がある。2022年からAWSに機能要望が出ているが、2026年1月時点でも未対応。

この問題を解決するCLIツール `ssmx` を作成する。

## 技術方針

- **言語**: Go
- **SSMプロトコル実装**: [ssm-session-client](https://github.com/mmmorris1975/ssm-session-client) ライブラリを使用
  - AWS SDKでSSM StartSession APIを呼び出し、WebSocket接続を直接管理する
  - `session-manager-plugin` への依存を排除し、キープアライブ間隔を自由に制御可能にする
- **インスタンス一覧取得**: AWS SDK for Go v2 (`aws-sdk-go-v2`) を使用

## 機能要件

### 1. インタラクティブなインスタンス選択

- `aws ec2 describe-instances` 相当のAPIでインスタンス一覧を取得
- **fzf** ライクなインタラクティブUIで対象インスタンスを選択できること
  - 表示項目: InstanceId, PrivateIpAddress, Name タグ
  - 外部の `fzf` コマンドへの依存ではなく、Goライブラリで実装する（例: [go-fuzzyfinder](https://github.com/ktr0731/go-fuzzyfinder)）
- Running状態のインスタンスのみ表示

### 2. AWS プロファイル指定

- `-p` / `--profile` フラグでAWSプロファイルを指定可能
- デフォルトは `default` プロファイル
- `AWS_PROFILE` 環境変数にも対応

### 3. キープアライブ間隔の設定

- `--keepalive` / `-k` フラグでWebSocketのPing間隔を秒単位で指定可能
- デフォルト: `30` 秒
- 例: `ssmx -k 15` で15秒間隔

### 4. セッション接続

- 選択したインスタンスに対してSSMセッションを開始
- シェルセッション（`aws ssm start-session` 相当）を提供
- Ctrl+C やセッション終了時に適切にクリーンアップ

## CLI インターフェース

```
ssmx - AWS SSM Session Manager with configurable keepalive

Usage:
  ssmx [flags]

Flags:
  -p, --profile string      AWS profile to use (default "default")
  -k, --keepalive int       WebSocket keepalive interval in seconds (default 30)
  -r, --region string       AWS region
  -t, --target string       Instance ID (skip interactive selection)
  -h, --help                Show help
  -v, --version             Show version
```

### 使用例

```bash
# インタラクティブにインスタンスを選択して接続
ssmx

# プロファイル指定
ssmx -p production

# キープアライブ間隔を15秒に設定
ssmx -p production -k 15

# インスタンスIDを直接指定（fzf選択をスキップ）
ssmx -t i-0123456789abcdef0
```

## 依存ライブラリ（想定）

| ライブラリ | 用途 |
|-----------|------|
| `github.com/mmmorris1975/ssm-session-client` | SSMセッションプロトコル実装 |
| `github.com/aws/aws-sdk-go-v2` | AWS API呼び出し |
| `github.com/ktr0731/go-fuzzyfinder` | fzfライクなインタラクティブ選択 |
| `github.com/spf13/cobra` | CLIフレームワーク |

## 既存の類似ツール（参考）

| ツール | 状態 | キープアライブ対応 |
|--------|------|-------------------|
| [gossm](https://github.com/gjbae1212/gossm) | 2025年4月アーカイブ済み | なし |
| [aws-gate](https://github.com/xen0l/aws-gate) | Python | なし |
| [aws-ssm-tools](https://github.com/mludvig/aws-ssm-tools) | Python | なし |

**いずれもキープアライブに対応しておらず、ssmx が解決する差別化ポイントとなる。**

## 配布

- GitHub Actionsを使用
- GitHub Releases でバイナリ配布（darwin/arm64, darwin/amd64, linux/amd64）
- Homebrew tap での配布も検討
