package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"github.com/touilleio/alephium-go-client"
	"math/rand"
	"strings"
	"time"
)

type miningHandler struct {
	alephiumClient           *alephium.Client
	walletName               string
	walletPassword           string
	walletMnemonic           string
	walletMnemonicPassphrase string
	printMnemonic            bool
	log                      *logrus.Logger
}

func newMiningHandler(alephiumClient *alephium.Client, walletName string, walletPassword string,
	walletMnemonic string, walletMnemonicPassphrase string, printMnemonic bool,
	log *logrus.Logger) (*miningHandler, error) {

	handler := &miningHandler{
		alephiumClient:           alephiumClient,
		walletName:               walletName,
		walletPassword:           walletPassword,
		walletMnemonic:           walletMnemonic,
		walletMnemonicPassphrase: walletMnemonicPassphrase,
		printMnemonic:            printMnemonic,
		log:                      log,
	}

	return handler, nil
}

func getAddressesInRandomOrder(walletAddresses alephium.WalletAddresses) []string {
	a := alephium.GetAddressesAsString(walletAddresses.Addresses)
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
			if walletAddress.Address == minerAddress {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (h *miningHandler) createAndUnlockWallet() (alephium.WalletInfo, error) {

	// Initialize wallet to locked, unlocking twice doesn't hurt much.
	wallet := alephium.WalletInfo{Wallet: alephium.Wallet{Name: h.walletName}, Locked: true}

	walletFound, err := h.alephiumClient.CheckWalletExist(h.walletName)
	if err != nil {
		h.log.Debugf("Got an error calling CheckWalletExist endpoint %s. Err = %v", h.alephiumClient, err)
		return wallet, err
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
				h.log.Infof("[SENSITIVE] The mnemonic of the newly created wallet is [ %s ]. This mnemonic will never be printed again, make sure you write them down somewhere!",
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
		ok, err := h.alephiumClient.UnlockWallet(h.walletName, h.walletPassword, h.walletMnemonicPassphrase)
		if err != nil {
			h.log.Debugf("Got an error while unlocking the wallet %s. Err = %v", h.walletName, err)
			return wallet, err
		}
		if !ok {
			h.log.Debugf("Unable to unlock the wallet %s, please make sure the provided password is correct and retry.", h.walletName)
			return wallet, err
		}
	}

	return wallet, nil
}

func (h *miningHandler) updateMinersAddresses() error {
	minerAddresses, err := h.alephiumClient.GetMinersAddresses()
	if err != nil && !strings.HasPrefix(err.Error(), "Miner addresses are not set up") {
		h.log.Debugf("Got an error calling miners addresses. Err = %v", err)
		return err
	}

	walletAddresses, err := h.alephiumClient.GetWalletAddresses(h.walletName)
	if err != nil {
		h.log.Debugf("Got an error calling wallet addresses. Err = %v", err)
		return err
	}

	if !hasSameAddresses(minerAddresses, walletAddresses) {
		h.log.Debugf("Current miner addresses %v", minerAddresses)
		h.log.Debugf("Mining wallet addresses %v", walletAddresses)

		err = h.alephiumClient.UpdateMinersAddresses(alephium.GetAddressesAsString(walletAddresses.Addresses))
		if err != nil {
			h.log.Debugf("Got an error calling update miners addresses. Err = %v", err)
			return err
		}
	}
	return nil
}

func (h *miningHandler) waitForNodeInSync() error {

	_, err := h.alephiumClient.WaitUntilSyncedWithAtLeastOnePeer(context.Background())
	if err != nil {
		h.log.Debugf("Got an error waiting for the node to be in sync with peers. Err = %v", err)
		return err
	}
	return nil
}

// ensureMiningWalletAndNodeMining
func (h *miningHandler) ensureMiningWalletAndNodeMining() error {
	for range time.Tick(5 * time.Minute) {
		err := h.updateMinersAddresses()
		if err != nil {
			h.log.Fatalf("Got an error while updating miners addresses. Err = %v", err)
			return err
		}
		err = h.waitForNodeInSync()
		if err != nil {
			h.log.Fatalf("Got an error while waiting for the node to be in sync with peers. Err = %v", err)
			return err
		}
	}
	return nil
}
