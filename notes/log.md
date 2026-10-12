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
