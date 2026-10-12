package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

var erroredUpstream = errors.New("errored upstream")

type paymentGatewayPostPaymentRequest struct {
	Amount int `json:"amount"`
}

type paymentGatewayGetPaymentsResponseOne struct {
	Amount int    `json:"amount"`
	Status string `json:"status"`
}

func requestPaymentGatewayPostPayment(ctx context.Context, paymentGatewayURL string, token string, param *paymentGatewayPostPaymentRequest, retrieveRidesOrderByCreatedAtAsc func() ([]Ride, error)) error {
	b, err := json.Marshal(param)
	if err != nil {
		return err
	}

	// 失敗したらとりあえずリトライ
	// FIXME: 社内決済マイクロサービスのインフラに異常が発生していて、同時にたくさんリクエストすると変なことになる可能性あり
	retry := 0
	for {
		err := func() error {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, paymentGatewayURL+"/payments", bytes.NewBuffer(b))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusNoContent {
				// エラーが返ってきても成功している場合があるので、社内決済マイクロサービスに問い合わせ
				getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, paymentGatewayURL+"/payments", bytes.NewBuffer([]byte{}))
				if err != nil {
					return err
				}
				getReq.Header.Set("Authorization", "Bearer "+token)

				getRes, err := http.DefaultClient.Do(getReq)
				if err != nil {
					return err
				}
				defer res.Body.Close()

				// GET /payments は障害と関係なく200が返るので、200以外は回復不能なエラーとする
				if getRes.StatusCode != http.StatusOK {
					return fmt.Errorf("[GET /payments] unexpected status code (%d)", getRes.StatusCode)
				}
				var payments []paymentGatewayGetPaymentsResponseOne
				if err := json.NewDecoder(getRes.Body).Decode(&payments); err != nil {
					return err
				}

				rides, err := retrieveRidesOrderByCreatedAtAsc()
				if err != nil {
					return err
				}

				if len(rides) != len(payments) {
					return fmt.Errorf("unexpected number of payments: %d != %d. %w", len(rides), len(payments), erroredUpstream)
				}

				return nil
			}
			return nil
		}()
		if err != nil {
			if retry < 5 {
				retry++
				time.Sleep(100 * time.Millisecond)
				continue
			} else {
				return err
			}
		}
		break
	}

	return nil
}

var (
	paymentStatsMu sync.Mutex
	paymentCount   int
	paymentRetries int
	paymentTotal   time.Duration
	paymentCodes   = map[int]int{}
	paymentCodeDur = map[int]time.Duration{}
	paymentReqDur  time.Duration
)

var paymentSem = make(chan struct{}, 10)

var paymentClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 500,
		IdleConnTimeout:     90 * time.Second,
	},
}

func paySync(paymentGatewayURL, token, rideID string, param *paymentGatewayPostPaymentRequest) error {
	b, err := json.Marshal(param)
	if err != nil {
		return err
	}
	start := time.Now()
	defer func() {
		paymentStatsMu.Lock()
		paymentCount++
		paymentTotal += time.Since(start)
		if paymentCount%500 == 0 {
			slog.Info("payment", "count", paymentCount, "avg_ms", paymentTotal.Milliseconds()/int64(paymentCount), "retries", paymentRetries, "codes", fmt.Sprint(paymentCodes), "durs", fmt.Sprint(paymentCodeDur), "req_avg_ms", paymentReqDur.Milliseconds()/int64(paymentCount+paymentRetries))
		}
		paymentStatsMu.Unlock()
	}()
	for i := 0; i < 1000; i++ {
		if i > 0 {
			paymentStatsMu.Lock()
			paymentRetries++
			paymentStatsMu.Unlock()
		}
		req, err := http.NewRequest(http.MethodPost, paymentGatewayURL+"/payments", bytes.NewBuffer(b))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Idempotency-Key", rideID)
		t0 := time.Now()
		res, err := paymentClient.Do(req)
		paymentStatsMu.Lock()
		if err == nil {
			paymentCodeDur[res.StatusCode] += time.Since(t0)
		}
		paymentReqDur += time.Since(t0)
		if err != nil {
			paymentCodes[-1]++
		}
		paymentStatsMu.Unlock()
		if err == nil {
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			paymentStatsMu.Lock()
			paymentCodes[res.StatusCode]++
			paymentStatsMu.Unlock()
			if res.StatusCode == http.StatusNoContent {
				return nil
			}
			if res.StatusCode == http.StatusBadRequest || res.StatusCode == http.StatusUnprocessableEntity {
				slog.Error("payment failed", "status", res.StatusCode, "ride", rideID)
				return fmt.Errorf("payment failed: %d", res.StatusCode)
			}
		}
	}
	return errors.New("payment retry exceeded")
}
