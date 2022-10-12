package main

import (
	"context"
	alephium "github.com/alephium/go-sdk"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type AddressBalanceStats struct {
	alephiumClient *alephium.APIClient
	addresses      []string
	metrics        *metrics
}

func newAddressBalanceStats(alephiumClient *alephium.APIClient, addresses []string, metrics *metrics) (*AddressBalanceStats, error) {
	handler := &AddressBalanceStats{
		alephiumClient: alephiumClient,
		addresses:      addresses,
		metrics:        metrics,
	}
	return handler, nil
}

func (h *AddressBalanceStats) Stats(ctx context.Context) error {
	err := h.doStats(ctx)
	if err != nil {
		return err
	}
	for range time.Tick(time.Minute) {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		err = h.doStats(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *AddressBalanceStats) doStats(ctx context.Context) error {
	for _, address := range h.addresses {
		addressBalanceReq := h.alephiumClient.AddressesApi.GetAddressesAddressBalance(ctx, address)
		balance, _, err := addressBalanceReq.Execute()
		if err != nil {
			return err
		}
		if addressBalance, ok := ALPHFromCoinString(balance.Balance); ok {
			h.metrics.addressTotalBalance.With(prometheus.Labels{"address": address}).Set(addressBalance.FloatALPH())
		}
		if addressLockedBalance, ok := ALPHFromCoinString(balance.LockedBalance); ok {
			h.metrics.addressLockedBalance.With(prometheus.Labels{"address": address}).Set(addressLockedBalance.FloatALPH())
		}
		h.metrics.addressUtxos.With(prometheus.Labels{"address": address}).Set(float64(balance.UtxoNum))
	}
	return nil
}
