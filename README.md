# isucon14-baseline

ISUCON14 の競技環境。

- 問題の資料: `docs/`（当日マニュアル・アプリケーションマニュアル）
- 競技のルール: `CLAUDE.md`
- 環境: EC2（競技サーバー c5.large × 3 + ベンチ）

## 進め方（人間がやること）

```sh
aws sso login --profile <profile>
infra/up.sh       # EC2 を作り、ssh_config と BENCH.md を書く（課金開始）
# … 競技 …
infra/down.sh     # 必ず消す
```

EC2 の作成・破棄は競技をするセッションにはやらせない。
