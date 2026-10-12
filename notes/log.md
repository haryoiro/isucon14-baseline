# 作業ログ

- 開始: 1791806481 (21:01) / 160分 = 23:41 / 180分 = 00:01
- 21:03 初期ベンチ 1232。コードを手元に取り込み、deploy/bench スクリプトを作成
- 21:06 インデックス追加、chair_totals テーブル（総走行距離・最新位置）、getChairStats を集約1クエリ化、nearby を1クエリ化、マッチングを「待ちライド全件 × 空き椅子の最近傍（距離/速度）」に変更、chi Logger と nginx access log を off、MySQL 設定（buffer pool 1G・flush=2・binlog off）、通知 retry 200ms、matcher 0.2s → 28780
- 21:15 MySQL を isu2 に分離（isu1 の mysql は disable、isuride-go.service から Requires=mysql を削除、env.sh の DB_HOST=192.168.0.12）→ 57137
- 21:20 通知を SSE 化（in-memory hub で状態遷移時に push）、認証のトークン→エンティティをキャッシュ、nginx upstream keepalive・worker_rlimit_nofile 65535（初回は Too many open files で fail）→ 111742
- 21:26 椅子の状態（位置・総距離・現ライド・active）をメモリに持ち、chair_locations/chair_totals は 100ms ごとにバルク書き込み。マッチングをアプリ内 goroutine（100ms）にして isuride-matcher を disable → 208059
- 21:32 app を isu3 に移動（isu1=nginx, isu2=MySQL, isu3=app）。isu1 の nginx+app で CPU が張り付いていたため → 217710（CPU はどこも余裕あり＝ゲーム側の律速）
- 21:35 マッチングを全ペア貪欲にすると乗車地点・移動の不満は激減したが古いライドが飢えて fail(CODE=32)。5秒以上待ちを FIFO 優先にするハイブリッドへ
- 21:38 nearby の CODE=31（椅子が足りない）は、ベンチが「MATCHING 中（未受理）の椅子」を空きとして扱っているためと判明。nearby に含める → 247650
- 21:44 決済を commit 後に同期実行（Idempotency-Key=ride_id でリトライ、keepalive 付き専用 http.Client）。非同期にすると CODE=34 で fail したので同期のまま → 388436
- 21:51 ride キャッシュ（ride・fare・最新 status・椅子評価統計）で publish の DB クエリを除去 → 462086(fail: nearby CODE=30)。nearby は「MATCHING 通知前の椅子は含める」「解放後1秒は除外」でエラー解消 → 427254
- 22:03 マッチングの urgentWait を 0/3/15 秒で比較（15s: 269k, 0: 349k, 3s: 400-466k）→ 3s。決済を commit 前に戻して owner sales の CODE=24 を解消 → 458160
- 22:21 ベンチサーバーが c5.xlarge→c5.2xlarge に変更された（運営側のお知らせ）。以降のスコアは以前と直接比較しない
- 22:27 新ベンチで isu1 nginx と isu2 MySQL が飽和。静的ファイルを事前 gzip して gzip_static on（nginx 195%→162%）、upstream keepalive 1024、GOGC=200 → 600k 前後
