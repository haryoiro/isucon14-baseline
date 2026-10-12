# isucon14-baseline

**ISUCON14 を、事前に用意した知識（テンプレート・スキル・手順集・ノウハウ・ネタバレ）なしで Claude Code に解かせる**ための環境。

## 入っているもの / 入っていないもの

| 入っている | 入っていない |
|---|---|
| 当日マニュアル・アプリケーションマニュアル（`docs/`。本番でも配られる） | 講評・解説・参加記・上位の解法 |
| 競技環境（EC2: c5.large × 3 + bench c5.xlarge。前回と同じ） | isucon-template（スキル・手順集・ノウハウ・計測の道具） |
| 競技のルール（`CLAUDE.md`） | isuspan などの計測ツール |

残る偏り: モデル自身が学習で ISUCON14 を知っている可能性は消せない。

## 知識の入り口を塞ぐ設定（`.claude/settings.json`）

Web 検索・Web 取得・`gh`・`aws`・`terraform` を禁止し、隣の ISUCON 関係のディレクトリ（isucon-spoilers / isucon-suburi / isucon-template / isuspan / isucon-tooling-lab）とメモリの読み取りを禁止している。
**シェル経由の読み取り（`cat` など）まで完全に防げる保証はない**ので、`CLAUDE.md` でもルールとして禁止している。より確実にするなら、このリポジトリだけをマウントしたコンテナの中で動かす。

## 進め方（人間がやること）

```sh
cp ../isucon-suburi/terraform/terraform.tfvars infra/terraform.tfvars   # 鍵と AWS の profile（未作成なら作る）
aws sso login --profile waroka
infra/up.sh            # EC2 を作り、ssh_config と BENCH.md を書く（課金開始）
claude --dangerously-skip-permissions   # 別ターミナル。下の「最初の指示」を渡す
# … 3時間後 …
infra/down.sh          # 必ず消す
```

EC2 の作成・破棄は bypass のセッションにはやらせない（AWS の権限を持たせないため）。
