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
