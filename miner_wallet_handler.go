package main

import (
	"github.com/sirupsen/logrus"
	"github.com/touilleio/alephium-go-client"
	"math/rand"
)

type minerWalletHandler struct {
	alephiumClient *alephium.AlephiumClient
	walletName string
	walletPassword string
	walletMnemonic string
	walletMnemonicPassphrase string
	printMnemonic bool
	log *logrus.Logger
}

func newMinerWalletHandler(alephiumClient *alephium.AlephiumClient, walletName string, walletPassword string,
	walletMnemonic string, walletMnemonicPassphrase string, printMnemonic bool,
	log *logrus.Logger) (*minerWalletHandler, error) {

	handler := &minerWalletHandler{
		alephiumClient: alephiumClient,
		walletName: walletName,
		walletPassword: walletPassword,
		walletMnemonic: walletMnemonic,
		walletMnemonicPassphrase: walletMnemonicPassphrase,
		printMnemonic: printMnemonic,
		log: log,
	}

	return handler, nil
}

func getAddressesInRandomOrder(walletAddresses alephium.WalletAddresses) []string {
	a := walletAddresses.Addresses
	rand.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
	return a
}

func hasSameAddresses(minerAddresses alephium.MinersAddresses, walletAddresses alephium.WalletAddresses) bool {

	if len(minerAddresses.Addresses) <= 1 || len(minerAddresses.Addresses) != len(walletAddresses.Addresses) {
		return false
	}

	for _, minerAddress := range minerAddresses.Addresses {
		found := false
		for _, walletAddress := range walletAddresses.Addresses {
			if walletAddress == minerAddress {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (h *minerWalletHandler) createAndUnlockWallet() (alephium.WalletInfo, error) {

	var wallet alephium.WalletInfo

	var walletFound bool
	wallets, err := h.alephiumClient.GetWallets()
	if err != nil {
		h.log.Debugf("Got an error calling wallets endpoint %s. Err = %v", h.alephiumClient, err)
		return wallet, err
	}

	for _, w := range wallets {
		if w.Name == h.walletName {
			wallet = w
			walletFound = true
			break
		}
	}

	if !walletFound {
		h.log.Infof("Wallet %s not found, creating or restoring it now.", h.walletName)
		if h.walletMnemonic != "" {
			restoredWallet, err := h.alephiumClient.RestoreWallet(h.walletPassword, h.walletMnemonic,
				h.walletName, true, h.walletMnemonicPassphrase)
			if err != nil {
				h.log.Debugf("Got an error calling wallet restore endpoint %v. Err = %v", h.alephiumClient, err)
				return wallet, err
			}

			wallet, err = h.alephiumClient.GetWalletStatus(restoredWallet.Name)
			if err != nil {
				h.log.Debugf("Got an error calling wallet status after a restore, wallet restoration probably didn't work... Err = %v", err)
				return wallet, err
			}
		} else {
			createdWallet, err := h.alephiumClient.CreateWallet(h.walletName, h.walletPassword,
				true, h.walletMnemonicPassphrase)
			if err != nil {
				h.log.Debugf("Got an error calling wallet create endpoint %v. Err = %v", h.alephiumClient, err)
				return wallet, err
			}
			if h.printMnemonic {
				h.log.Infof("[SENSITIVE] The mnemonic of the newly created wallet is %s. This mnemonic will never be printed again, make sure you write them down somewhere!",
					createdWallet.Mnemonic)
			}
			wallet, err = h.alephiumClient.GetWalletStatus(createdWallet.Name)
			if err != nil {
				h.log.Debugf("Got an error calling wallet status after a create, wallet creation probably didn't work... Err = %v", err)
				return wallet, err
			}
		}
	}

	if wallet.Locked {
		ok, err := h.alephiumClient.UnlockWallet(wallet.Name, h.walletPassword)
		if err != nil {
			h.log.Debugf("Got an error while unlocking the wallet %s. Err = %v", wallet.Name, err)
			return wallet, err
		}
		if !ok {
			h.log.Debugf("Unable to unlock the wallet %s, please make sure the provided password is correct and retry.", wallet.Name)
			return wallet, err
		}
	}

	return wallet, nil
}

func (h *minerWalletHandler) updateMinersAddresses() error {
	minerAddresses, err := h.alephiumClient.GetMinersAddresses()
	if err != nil {
		h.log.Debugf("Got an error calling miners addresses. Err = %v", err)
		return err
	}
	walletAddresses, err := h.alephiumClient.GetWalletAddresses(h.walletName)
	if err != nil {
		h.log.Debugf("Got an error calling wallet addresses. Err = %v", err)
		return err
	}
	if !hasSameAddresses(minerAddresses, walletAddresses) {
		err = h.alephiumClient.UpdateMinersAddresses(walletAddresses.Addresses)
		if err != nil {
			h.log.Debugf("Got an error calling update miners addresses. Err = %v", err)
			return err
		}
	}
	return nil
}
