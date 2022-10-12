package main

import (
	"context"
	"fmt"
	alephium "github.com/alephium/go-sdk"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type transferHandler struct {
	alephiumClient     *alephium.APIClient
	walletName         string
	walletPassword     string
	mnemonicPassphrase string
	transferAddress    string
	transferMinAmount  ALPH
	transferFrequency  time.Duration
	immediate          bool
	metrics            *metrics
	log                *logrus.Logger
	concurrentExecLock *sync.RWMutex
}

func newTransferHandler(alephiumClient *alephium.APIClient, walletName string, walletPassword string,
	mnemonicPassphrase string, transferAddress string, transferMinAmount string, transferFrequency time.Duration,
	immediate bool, metrics *metrics, log *logrus.Logger) (*transferHandler, error) {

	minAlf, ok := ALPHFromCoinString(transferMinAmount)
	if !ok {
		return nil, fmt.Errorf("transferMinAmount %s is not a valid ALPH transfer amoount", transferMinAmount)
	}

	handler := &transferHandler{
		alephiumClient:     alephiumClient,
		walletName:         walletName,
		walletPassword:     walletPassword,
		mnemonicPassphrase: mnemonicPassphrase,
		transferAddress:    transferAddress,
		transferMinAmount:  minAlf,
		transferFrequency:  transferFrequency,
		immediate:          immediate,
		metrics:            metrics,
		log:                log,
		concurrentExecLock: &sync.RWMutex{},
	}

	return handler, nil
}

func (h *transferHandler) handle(ctx context.Context, log *logrus.Entry) error {
	if h.immediate {
		err := h.transfer(ctx, log)
		if err != nil {
			h.log.Debugf("Got an error while immediately transferring some amount. Err = %v", err)
			return err
		}
	}
	for range time.Tick(h.transferFrequency) {
		err := h.transfer(ctx, log)
		if err != nil {
			h.log.Debugf("Got an error while transferring some amount. Err = %v", err)
			return err
		}
	}
	return nil
}

func (h *transferHandler) transfer(ctx context.Context, log *logrus.Entry) error {

	locked := h.concurrentExecLock.TryLock()
	if !locked {
		log.Warnf("Another transfer process is still running...")
		return nil
	}
	defer h.concurrentExecLock.Unlock()

	h.metrics.transferRun.Inc()

	wallet, err := getWalletStatus(ctx, h.alephiumClient, h.walletName, log)
	if err != nil {
		h.log.WithError(err).Debugf("Got an error calling wallet status, err = %v", err)
		return err
	}
	if wallet.Locked {
		err := unlockWallet(ctx, h.alephiumClient, wallet.WalletName, h.walletPassword, h.mnemonicPassphrase, log)
		if err != nil {
			h.log.WithError(err).Debugf("Got an error calling wallet unlock. Err = %v", err)
			return err
		}
	}

	sweep := alephium.NewSweep(h.transferAddress)
	sweepAllReq := h.alephiumClient.WalletsApi.PostWalletsWalletNameSweepAllAddresses(ctx, wallet.WalletName).Sweep(*sweep)
	transferRes, _, err := sweepAllReq.Execute()
	if err != nil {
		h.log.WithError(err).Debugf("Got an error while sweeping all")
		return err
	}

	for _, tx := range transferRes.GetResults() {
		h.log.Infof("New tx %s,%d->%d just submitted", tx.TxId, tx.FromGroup, tx.ToGroup)
		var txConfirmed *alephium.Confirmed
		for txConfirmed == nil {

			txStatusReq := h.alephiumClient.TransactionsApi.GetTransactionsStatus(ctx)
			txStatus, _, err := txStatusReq.TxId(tx.TxId).ToGroup(tx.FromGroup).ToGroup(tx.ToGroup).Execute()
			if err != nil {
				h.log.WithError(err).Debugf("Got an error while getting tx status for tx %s", tx.TxId)
				return err
			}
			txConfirmed = txStatus.Confirmed
		}
		h.log.Infof("New tx %s,%d->%d is now inculded in block %s!", tx.TxId, tx.FromGroup, tx.ToGroup, txConfirmed.BlockHash)
		// TODO: get block to find out the exact amount transferred to update counter consistently
	}

	return nil
}

func roundAmount(amount ALPH, txMinAmount ALPH, txMaxAmount ALPH) ALPH {
	twiceMinAmount := txMinAmount.Multiply(2)
	if amount.Cmp(twiceMinAmount) >= 0 {
		txAmount := amount.Subtract(txMinAmount)
		if txAmount.Cmp(txMaxAmount) > 0 {
			return txMaxAmount
		} else {
			return txAmount
		}
	}
	return ALPH{}
}
