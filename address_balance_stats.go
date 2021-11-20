package main

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/touilleio/alephium-go-client"
	"time"
)

type AddressBalanceStats struct {
	alephiumClient *alephium.Client
	addresses []string
	metrics *metrics
}

func newAddressBalanceStats(alephiumClient *alephium.Client, addresses []string, metrics *metrics) (*AddressBalanceStats, error) {
	handler := &AddressBalanceStats{
		alephiumClient: alephiumClient,
		addresses: addresses,
		metrics: metrics,
	}
	return handler, nil
}

func (h *AddressBalanceStats) Stats(ctx context.Context) error {
	err := h.doStats()
	if err != nil {
		return err
	}
	for range time.Tick(time.Minute) {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		err = h.doStats()
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *AddressBalanceStats) doStats() error {
	for _, address := range h.addresses {
		balance, err := h.alephiumClient.GetAddressBalance(address, -1)
		if err != nil {
			return err
		}
		h.metrics.addressTotalBalance.With(prometheus.Labels{"address": address}).Set(balance.Balance.FloatALPH())
		h.metrics.addressLockedBalance.With(prometheus.Labels{"address": address}).Set(balance.LockedBalance.FloatALPH())
	}
	return nil
}
