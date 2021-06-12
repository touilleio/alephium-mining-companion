package main

import (
	"github.com/sirupsen/logrus"
	"github.com/touilleio/alephium-go-client"
	"math/big"
	"time"
)

type transferHandler struct {
	alephiumClient    *alephium.Client
	walletName        string
	walletPassword    string
	transferAddress   string
	transferMaxAmount string
	transferFrequency time.Duration
	immediate         bool
	metrics           *metrics
	log               *logrus.Logger
}

func newTransferHandler(alephiumClient *alephium.Client, walletName string, walletPassword string,
	transferAddress string, transferMaxAmount string, transferFrequency time.Duration, immediate bool, metrics *metrics, log *logrus.Logger) (*transferHandler, error) {

	handler := &transferHandler{
		alephiumClient:    alephiumClient,
		walletName:        walletName,
		walletPassword:    walletPassword,
		transferAddress:   transferAddress,
		transferMaxAmount: transferMaxAmount,
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

	walletBalances, err := h.alephiumClient.GetWalletBalances(wallet.Name)
	if err != nil {
		h.log.Debugf("Got an error while getting wallet balances. Err = %v", err)
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
		amount := getAmount(address, walletBalances.Balances)
		h.log.Debugf("address %s, amount %s", address, amount)
		if amount != "" {
			roundAmount := roundAmount(amount, h.transferMaxAmount)
			h.log.Debugf("%s, %s, %s", address, amount, roundAmount)
			if roundAmount != "" {
				tx, err := h.alephiumClient.Transfer(wallet.Name, h.transferAddress, roundAmount)
				if err != nil {
					h.log.Debugf("Got an error calling transfer. Err = %v", err)
					return err
				}
				h.log.Debugf("New tx %s,%d->%d of %s from %s to %s", tx.TransactionId, tx.FromGroup, tx.ToGroup,
					roundAmount, address, h.transferAddress)
			}
		}
	}
	return nil
}

func getAmount(address string, balances []alephium.AddressBalance) string {
	for _, b := range balances {
		if b.Address == address {
			return b.Balance
		}
	}
	return ""
}

var (
	OneALP      = "1000000000000000000"
	TwoALP      = "2000000000000000000"
	TenALP      = "10000000000000000000"
	HundredALP  = "100000000000000000000"
	ThousandALP = "1000000000000000000000"
	two         = big.NewInt(2)
)

func roundAmount(amount string, txAmount string) string {
	balance, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return ""
	}
	limit, _ := new(big.Int).SetString(TenALP, 10)
	if balance.Cmp(limit) > 0 {
		limit = limit.Div(limit, two).Neg(limit)
		balance.Add(balance, limit)
		alp, _ := new(big.Int).SetString(txAmount, 10)
		if balance.Cmp(alp) > 0 {
			return alp.Text(10)
		}
		return balance.Text(10)
	}
	return ""
}
