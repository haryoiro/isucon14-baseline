# 作業ログ

- 開始: 1791806481 (21:01) / 160分 = 23:41 / 180分 = 00:01
- 21:03 初期ベンチ 1232。コードを手元に取り込み、deploy/bench スクリプトを作成
- 21:06 インデックス追加、chair_totals テーブル（総走行距離・最新位置）、getChairStats を集約1クエリ化、nearby を1クエリ化、マッチングを「待ちライド全件 × 空き椅子の最近傍（距離/速度）」に変更、chi Logger と nginx access log を off、MySQL 設定（buffer pool 1G・flush=2・binlog off）、通知 retry 200ms、matcher 0.2s → 28780
- 21:15 MySQL を isu2 に分離（isu1 の mysql は disable、isuride-go.service から Requires=mysql を削除、env.sh の DB_HOST=192.168.0.12）→ 57137
