package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/touilleio/alephium-go-client"
	"time"
)

type transferHandler struct {
	alephiumClient    *alephium.Client
	walletName        string
	walletPassword    string
	transferAddress   string
	transferMinAmount alephium.ALF
	transferMaxAmount alephium.ALF
	transferFrequency time.Duration
	immediate         bool
	metrics           *metrics
	log               *logrus.Logger
}

func newTransferHandler(alephiumClient *alephium.Client, walletName string, walletPassword string,
	transferAddress string, transferMinAmount string, transferMaxAmount string, transferFrequency time.Duration, immediate bool, metrics *metrics, log *logrus.Logger) (*transferHandler, error) {

	minAlf, ok := alephium.ALFromCoinString(transferMinAmount)
	if !ok {
		return nil, fmt.Errorf("transferMinAmount %s is not a valid ALF transfer amoount", transferMinAmount)
	}

	maxAlf, ok := alephium.ALFromCoinString(transferMaxAmount)
	if !ok {
		return nil, fmt.Errorf("transferMaxAmount %s is not a valid ALF transfer amoount", transferMaxAmount)
	}
	if maxAlf.Cmp(minAlf) < 0 {
		return nil, fmt.Errorf("transferMaxAmount %s must be bigger or equals to transferMinAmount %s", transferMaxAmount, transferMinAmount)
	}

	handler := &transferHandler{
		alephiumClient:    alephiumClient,
		walletName:        walletName,
		walletPassword:    walletPassword,
		transferAddress:   transferAddress,
		transferMinAmount: minAlf,
		transferMaxAmount: maxAlf,
		transferFrequency: transferFrequency,
		immediate:         immediate,
		metrics:           metrics,
		log:               log,
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
		h.log.Debugf("Got an error calling wallet status after a restore, wallet restoration probably didn't work... Err = %v", err)
		return err
	}
	if wallet.Locked {
		_, err := h.alephiumClient.UnlockWallet(wallet.Name, h.walletPassword)
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

		addressBalance, err := h.alephiumClient.GetAddressBalance(address)
		if err != nil {
			h.log.Debugf("Got an error while getting address balance. Err = %v", err)
			return err
		}

		amount := addressBalance.Balance.Subtract(addressBalance.LockedBalance)
		h.log.Debugf("Address %s has %s available (i.e. not locked)", address, amount.PrettyString())
		if amount.Amount == nil {
			continue
		}
		roundAmount := roundAmount(amount, h.transferMinAmount, h.transferMaxAmount)
		if roundAmount.Amount == nil {
			continue
		}
		h.log.Debugf("Will transfer from address %s, with available amount %s, effective rounded amount %s", address, amount.PrettyString(), roundAmount.PrettyString())

		tx, err := h.alephiumClient.Transfer(wallet.Name, h.transferAddress, roundAmount)
		if err != nil {
			h.log.Debugf("Got an error calling transfer. Err = %v", err)
			return err
		}
		h.log.Infof("New tx %s,%d->%d of %s from %s to %s", tx.TransactionId, tx.FromGroup, tx.ToGroup,
			roundAmount.PrettyString(), address, h.transferAddress)
	}
	return nil
}

func roundAmount(amount alephium.ALF, txMinAmount alephium.ALF, txMaxAmount alephium.ALF) alephium.ALF {
	twiceMinAmount := txMinAmount.Multiply(2)
	if amount.Cmp(twiceMinAmount) >= 0 {
		txAmount := amount.Subtract(txMinAmount)
		if txAmount.Cmp(txMaxAmount) > 0 {
			return txMaxAmount
		} else {
			return txAmount
		}
	}
	return alephium.ALF{}
}
