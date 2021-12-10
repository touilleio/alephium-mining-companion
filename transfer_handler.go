package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/touilleio/alephium-go-client"
	"time"
)

type transferHandler struct {
	alephiumClient     *alephium.Client
	walletName         string
	walletPassword     string
	mnemonicPassphrase string
	transferAddress    string
	transferMinAmount  alephium.ALPH
	transferFrequency  time.Duration
	immediate          bool
	metrics            *metrics
	log                *logrus.Logger
}

func newTransferHandler(alephiumClient *alephium.Client, walletName string, walletPassword string,
	mnemonicPassphrase string, transferAddress string, transferMinAmount string, transferFrequency time.Duration,
	immediate bool, metrics *metrics, log *logrus.Logger) (*transferHandler, error) {

	minAlf, ok := alephium.ALPHFromCoinString(transferMinAmount)
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
	}

	return handler, nil
}

func (h *transferHandler) handle() error {
	if h.immediate {
		err := h.transfer()
		if err != nil {
			h.log.Debugf("Got an error while immediately transferring some amount. Err = %v", err)
			return err
		}
	}
	for range time.Tick(h.transferFrequency) {
		err := h.transfer()
		if err != nil {
			h.log.Debugf("Got an error while transferring some amount. Err = %v", err)
			return err
		}
	}
	return nil
}

func (h *transferHandler) transfer() error {

	h.metrics.transferRun.Inc()

	wallet, err := h.alephiumClient.GetWalletStatus(h.walletName)
	if err != nil {
		h.log.Debugf("Got an error calling wallet status, err = %v", err)
		return err
	}
	if wallet.Locked {
		_, err := h.alephiumClient.UnlockWallet(wallet.Name, h.walletPassword, h.mnemonicPassphrase)
		if err != nil {
			h.log.Debugf("Got an error calling wallet unlock. Err = %v", err)
			return err
		}
	}

	walletAddresses, err := h.alephiumClient.GetWalletAddresses(wallet.Name)
	if err != nil {
		h.log.Debugf("Got an error while getting wallet addresses. Err = %v", err)
		return err
	}

	for _, address := range getAddressesInRandomOrder(walletAddresses) {

		ok, err := h.alephiumClient.ChangeActiveAddress(wallet.Name, address)
		if err != nil {
			h.log.Debugf("Got an error calling change active address. Err = %v", err)
			return err
		}
		if !ok {
			h.log.Warnf("Got a false while calling change active address. Not sure what this means yet...")
		}

		addressBalance, err := h.alephiumClient.GetAddressBalance(address, -1)
		if err != nil {
			h.log.Debugf("Got an error while getting address balance. Err = %v", err)
			return err
		}

		amount := addressBalance.Balance.Subtract(addressBalance.LockedBalance)
		h.log.Debugf("Address %s has %s available (i.e. not locked)", address, amount.PrettyString())
		if amount.Amount == nil {
			continue
		}
		if amount.Cmp(h.transferMinAmount) < 0 {
			h.log.Debugf("Available amount %s is not above the threshold %s", amount.PrettyString(), h.transferMinAmount.PrettyString())
			continue
		}

		h.log.Debugf("Will transfer all available (not locked) amount ~%s from address %s to %s", amount.PrettyString(), address, h.transferAddress)

		tx, err := h.alephiumClient.SweepAll(wallet.Name, h.transferAddress)
		if err != nil {
			h.log.Debugf("Got an error calling transfer. Err = %v", err)
			return err
		}
		h.metrics.txAmount.Add(amount.FloatALPH())
		h.log.Infof("New tx %s,%d->%d of ~%s from %s to %s", tx.TransactionId, tx.FromGroup, tx.ToGroup,
			amount.PrettyString(), address, h.transferAddress)
	}
	return nil
}

func roundAmount(amount alephium.ALPH, txMinAmount alephium.ALPH, txMaxAmount alephium.ALPH) alephium.ALPH {
	twiceMinAmount := txMinAmount.Multiply(2)
	if amount.Cmp(twiceMinAmount) >= 0 {
		txAmount := amount.Subtract(txMinAmount)
		if txAmount.Cmp(txMaxAmount) > 0 {
			return txMaxAmount
		} else {
			return txAmount
		}
	}
	return alephium.ALPH{}
}
