# スコア

| 時刻 | スコア | pass | 何を変えた後か |
|---|---|---|---|
| 21:03 | 1232 | true | 初期状態 |
| 21:06 | 28780 | true | index・chair_totals・マッチング一括最近傍・ログ off・mysql 設定 |
| 21:08 | 27223 | true | 同上(プロファイル取得) |
| 21:09 | 37046 | true | 同上(slowlog取得) |
| 21:11 | 28697 | true | 同上(slowlog取得2) |
| 21:13 | 27374 | true | 同上(slowlog取得3) |
| 21:14 |  | DB を isu2 に分離 |
| 21:15 | 57137 | true | DB を isu2 に分離 |
| 21:18 | 85051 | false | SSE 通知・認証キャッシュ |
| 21:20 | 111742 | true | SSE + nofile 上げ |
| 21:21 | 109721 | true | nearby retrieved_at 修正 (slowlog) |
| 21:26 | 208059 | true | 椅子状態をメモリ化（位置・距離・現ライド・active）、位置はバッチ書き込み、マッチングをアプリ内 goroutine 100ms |
| 21:28 | 173554 | true | nearby で解放直後1秒の椅子を除外 (slowlog) |
| 21:30 | 211801 | true | nearby: retrieved_at をスナップショット後に・椅子解放を評価レスポンス送信後に |
| 21:32 | 217710 | true | app を isu3 に移動（isu1=nginx, isu2=DB, isu3=app）、TLS session cache |
| 21:33 | 209512 | true | 同上（CPU 観測） |
| 21:35 | 123204 | false | マッチング: 全ペアを到着時間でソートして貪欲 |
| 21:36 | 211774 | false | マッチング: 5秒以上待ちは FIFO 最寄り優先 + 残りは全ペア貪欲 |
| 21:38 | 247650 | true | nearby に MATCHING 中の椅子も含める（実験） |
| 21:39 | 250184 | true | urgentWait 3s |
| 21:41 | 238145 | true | 同上（access log 計測） |
| 21:42 | 145 | false | 決済を非同期化（Idempotency-Key でリトライ） |
| 21:44 | 388436 | true | 決済: commit 後に同期実行・Idempotency-Key・keepalive client |
| 21:45 | 358122 | true | 同上（slowlog） |
| 21:48 | 462086 | false | ride キャッシュ（ride・fare・status・椅子評価統計をメモリ化） |
| 21:49 | 475250 | false | nearby: MATCHING 通知送信前の椅子のみ含める |
| 21:51 | 427254 | true | nearby: 解放後1秒は除外 |
| 21:53 | 353465 | true | 招待登録の FOR UPDATE をアプリ内 mutex に（deadlock 解消） |
| 21:54 | 466102 | true | 同上（再計測） |
| 21:55 | 456521 | true | 同上（CPU/latency 観測） |
| 21:57 | 402518 | true | 同上（slowlog） |
| 21:59 | 411224 | true | 同上（matching ログ） |
| 22:00 | 269201 | true | urgentWait 15s |
| 22:02 | 349531 | true | urgentWait 0（純 FIFO 最寄り） |
| 22:03 | 458160 | true | urgentWait 3s に戻す・決済を commit 前に（sales 整合） |
| 22:06 | 436349 | true | owner sales をメモリから計算・username UNIQUE 削除 |
| 22:07 | 445581 | true | chair_locations の永続化をやめる（chair_totals のみ）・nginx open_file_cache |
| 22:09 | 455665 | true | 同上（決済計測） |
| 22:11 | 446508 | true | 同上（決済ステータス計測） |
| 22:12 | 295802 | true | 決済の同時実行数を 10 に制限（実験） |
| 22:14 | 458225 | true | 決済リトライの sleep を除去 |
| 22:15 | 479565 | true | 同上（upstream 時間計測） |
| 22:17 | 464663 | true | マッチング計算を stMu の外で |
| 22:19 | 492652 | true | 同上（pprof） |
| 22:20 | 500144 | true | 同上（coord 計測） |
| 22:21 | --- | --- | **ここからベンチサーバーが c5.2xlarge に変更（運営側作業）。これ以前のスコアとは直接比較しない** |
| 22:24 | 607737 | true | upstream keepalive 1024・GOGC=200（新ベンチ） |
| 22:25 | 651263 | true | 同上（CPU 観測） |
| 22:27 | 599840 | true | 静的ファイルを事前 gzip（gzip_static） |
| 22:28 | 524026 | false | 同上（slowlog） |
| 22:30 | 594418 | true | 招待ロックを招待コード単位に |
