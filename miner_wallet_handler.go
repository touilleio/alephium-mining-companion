package main

import (
	"context"
	alephium "github.com/alephium/go-sdk"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

type miningHandler struct {
	alephiumClient           *alephium.APIClient
	walletName               string
	walletPassword           string
	walletMnemonic           string
	walletMnemonicPassphrase string
	printMnemonic            bool
	log                      *logrus.Logger
}

func newMiningHandler(alephiumClient *alephium.APIClient, walletName string, walletPassword string,
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

func hasSameAddresses(minerAddresses *alephium.MinerAddresses, walletAddresses *alephium.Addresses) bool {

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

func (h *miningHandler) createAndUnlockWallet(ctx context.Context, log *logrus.Entry) (*alephium.WalletStatus, error) {

	walletFound, err := checkWalletExist(ctx, h.alephiumClient, h.walletName, log)
	if err != nil {
		h.log.WithError(err).Debugf("Got an error calling CheckWalletExist endpoint %s", h.alephiumClient.GetConfig().Host)
		return nil, err
	}

	var wallet *alephium.WalletStatus
	if !walletFound {
		h.log.Infof("Wallet %s not found, creating or restoring it now.", h.walletName)
		if h.walletMnemonic != "" {

			restoredWallet, err := restoreWallet(ctx, h.alephiumClient, h.walletName, h.walletPassword, h.walletMnemonic,
				h.walletMnemonicPassphrase, true, log)
			if err != nil {
				h.log.WithError(err).Debugf("Got an error calling wallet restore endpoint %v", h.alephiumClient.GetConfig().Host)
				return nil, err
			}

			wallet, err = getWalletStatus(ctx, h.alephiumClient, restoredWallet.WalletName, log)
			if err != nil {
				h.log.WithError(err).Debugf("Got an error calling wallet status after a restore, wallet restoration probably didn't work...")
				return nil, err
			}
		} else {

			createdWallet, err := createWallet(ctx, h.alephiumClient, h.walletName, h.walletPassword,
				h.walletMnemonicPassphrase, true, log)

			if err != nil {
				h.log.WithError(err).Debugf("Got an error calling wallet create endpoint %v. Err = %v", h.alephiumClient.GetConfig().Host)
				return nil, err
			}
			if h.printMnemonic {
				h.log.Infof("[SENSITIVE] The mnemonic of the newly created wallet is [ %s ]. This mnemonic will never be printed again, make sure you write them down somewhere!",
					createdWallet.Mnemonic)
			}
			wallet, err = getWalletStatus(ctx, h.alephiumClient, createdWallet.WalletName, log)
			if err != nil {
				h.log.WithError(err).Debugf("Got an error calling wallet status after a create, wallet creation probably didn't work...")
				return nil, err
			}
		}
	} else {
		wallet, err = getWalletStatus(ctx, h.alephiumClient, h.walletName, log)
		if err != nil {
			h.log.WithError(err).Debugf("Got an error calling wallet status...")
			return nil, err
		}
	}

	if wallet.Locked {
		err := unlockWallet(ctx, h.alephiumClient, h.walletName, h.walletPassword, h.walletMnemonicPassphrase, log)
		if err != nil {
			h.log.WithError(err).Debugf("Got an error while unlocking the wallet %s", h.walletName)
			return wallet, err
		}
	}
	return wallet, nil
}

func (h *miningHandler) updateMinersAddresses(ctx context.Context, log *logrus.Entry) error {
	minerAddresses, err := getMinersAddresses(ctx, h.alephiumClient, log)
	if err != nil && !strings.HasPrefix(err.Error(), "Miner addresses are not set up") {
		h.log.WithError(err).Debugf("Got an error calling miners addresses. Err = %v", err)
		return err
	}

	walletAddresses, err := getWalletAddresses(ctx, h.alephiumClient, h.walletName, log)
	if err != nil {
		h.log.WithError(err).Debugf("Got an error calling wallet addresses")
		return err
	}

	if !hasSameAddresses(minerAddresses, walletAddresses) {
		h.log.Debugf("Current miner addresses %v", minerAddresses)
		h.log.Debugf("Mining wallet addresses %v", walletAddresses)

		err = updateMinerAddresses(ctx, h.alephiumClient, GetAddressesAsString(walletAddresses.Addresses), log)
		if err != nil {
			h.log.WithError(err).Debugf("Got an error calling update miners addresses")
			return err
		}
	}
	return nil
}

func (h *miningHandler) waitForNodeInSync(ctx context.Context, log *logrus.Entry) error {

	_, err := WaitUntilSyncedWithAtLeastOnePeer(ctx, h.alephiumClient, 30*time.Second, log)
	if err != nil {
		h.log.WithError(err).Debugf("Got an error waiting for the node to be in sync with peers")
		return err
	}
	return nil
}

// ensureMiningWalletAndNodeMining
func (h *miningHandler) ensureMiningWalletAndNodeMining(ctx context.Context, log *logrus.Entry) error {
	for range time.Tick(5 * time.Minute) {
		err := h.updateMinersAddresses(ctx, log)
		if err != nil {
			h.log.Fatalf("Got an error while updating miners addresses. Err = %v", err)
			return err
		}
		err = h.waitForNodeInSync(ctx, log)
		if err != nil {
			h.log.Fatalf("Got an error while waiting for the node to be in sync with peers. Err = %v", err)
			return err
		}
	}
	return nil
}

func checkWalletExist(ctx context.Context, alephiumClient *alephium.APIClient, walletName string, log *logrus.Entry) (bool, error) {
	walletNameRequest := alephiumClient.WalletsApi.GetWalletsWalletName(ctx, walletName)
	_, resp, err := walletNameRequest.Execute()

	if err != nil && (resp == nil || resp.StatusCode != http.StatusNotFound) {
		log.WithError(err).Debugf("Got an error while checking if the wallet %s exists", walletName)
		return false, err
	}

	if err != nil && resp != nil && resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return true, nil
}

func restoreWallet(ctx context.Context, alephiumClient *alephium.APIClient,
	walletName, walletPassword, walletMnemonic, walletMnemonicPassphrase string,
	isMiner bool, log *logrus.Entry) (*alephium.WalletRestoreResult, error) {

	walletRestore := alephium.NewWalletRestore(walletPassword, walletMnemonic, walletName)
	walletRestore.IsMiner = ToBoolPtr(isMiner)
	if walletMnemonicPassphrase != "" {
		walletRestore.MnemonicPassphrase = &walletMnemonicPassphrase
	}
	walletRestoreReq := alephiumClient.WalletsApi.PutWallets(ctx).WalletRestore(*walletRestore)
	walletRestoreRes, _, err := walletRestoreReq.Execute()
	if err != nil {
		log.WithError(err).Debugf("Got an error while restoring wallet %s", walletName)
		return nil, err
	}
	return walletRestoreRes, nil
}

func createWallet(ctx context.Context, alephiumClient *alephium.APIClient,
	walletName, walletPassword, walletMnemonicPassphrase string,
	isMiner bool, log *logrus.Entry) (*alephium.WalletCreationResult, error) {

	walletCreation := alephium.NewWalletCreation(walletPassword, walletName)
	walletCreation.IsMiner = ToBoolPtr(isMiner)
	if walletMnemonicPassphrase != "" {
		walletCreation.MnemonicPassphrase = &walletMnemonicPassphrase
	}
	walletCreationReq := alephiumClient.WalletsApi.PostWallets(ctx).WalletCreation(*walletCreation)
	walletCreationRes, _, err := walletCreationReq.Execute()
	if err != nil {
		log.WithError(err).Debugf("Got an error while restoring wallet %s", walletName)
		return nil, err
	}
	return walletCreationRes, nil
}

func ToBoolPtr(b bool) *bool {
	return &b
}

const (
	is404NotFound = "404 Not Found"
)

func unlockWallet(ctx context.Context, alephiumClient *alephium.APIClient,
	walletName, walletPassword, walletMnemonicPassphrase string,
	log *logrus.Entry) error {

	walletUnlock := alephium.NewWalletUnlock(walletPassword)
	if walletMnemonicPassphrase != "" {
		walletUnlock.MnemonicPassphrase = &walletMnemonicPassphrase
	}
	walletUnlockReq := alephiumClient.WalletsApi.PostWalletsWalletNameUnlock(ctx, walletName).
		WalletUnlock(*walletUnlock)
	_, err := walletUnlockReq.Execute()
	if err != nil {
		log.WithError(err).Debugf("Got an error while unlocking wallet %s", walletName)
		return err
	}
	return nil
}

func getWalletStatus(ctx context.Context, alephiumClient *alephium.APIClient,
	walletName string, log *logrus.Entry) (*alephium.WalletStatus, error) {

	walletNameRequest := alephiumClient.WalletsApi.GetWalletsWalletName(ctx, walletName)
	wallet, _, err := walletNameRequest.Execute()
	if err != nil {
		log.WithError(err).Debugf("Got an error while calling wallet status %s", walletName)
		return nil, err
	}
	return wallet, nil
}

func getWalletAddresses(ctx context.Context, alephiumClient *alephium.APIClient,
	walletName string, log *logrus.Entry) (*alephium.Addresses, error) {

	walletAddressesReq := alephiumClient.WalletsApi.GetWalletsWalletNameAddresses(ctx, walletName)
	walletAddresses, _, err := walletAddressesReq.Execute()
	if err != nil {
		log.WithError(err).Debugf("Got an error while calling get wallet addresses %s", walletName)
		return nil, err
	}
	return walletAddresses, nil
}

func getMinersAddresses(ctx context.Context, alephiumClient *alephium.APIClient,
	log *logrus.Entry) (*alephium.MinerAddresses, error) {

	minerAddressesReq := alephiumClient.MinersApi.GetMinersAddresses(ctx)
	minerAddresses, _, err := minerAddressesReq.Execute()
	if err != nil {
		log.WithError(err).Debugf("Got an error while calling miner addresses")
		return nil, err
	}
	return minerAddresses, nil
}

func updateMinerAddresses(ctx context.Context, alephiumClient *alephium.APIClient,
	addresses []string, log *logrus.Entry) error {
	minerAddresses := alephium.NewMinerAddresses(addresses)
	minerAddressesReq := alephiumClient.MinersApi.PutMinersAddresses(ctx).MinerAddresses(*minerAddresses)
	_, err := minerAddressesReq.Execute()
	if err != nil {
		log.WithError(err).Debugf("Got an error while updatring miner addresses")
		return err
	}
	return nil
}

func GetAddressesAsString(walletAddresses []alephium.AddressInfo) []string {
	addresses := make([]string, 0, len(walletAddresses))
	for _, wa := range walletAddresses {
		addresses = append(addresses, wa.Address)
	}
	return addresses
}

func getInterCliquePeerInfo(ctx context.Context, alephiumClient *alephium.APIClient, log *logrus.Entry) ([]alephium.InterCliquePeerInfo, error) {
	info, _, err := alephiumClient.InfosApi.GetInfosInterCliquePeerInfo(ctx).Execute()
	if err != nil {
		log.WithError(err).Debugf("Got an error while calling intercliquepeerinfo")
		return []alephium.InterCliquePeerInfo{}, err
	}
	return info, err
}

// IsSyncedWithAtLeastOnePeer checks if the clique is connected with at least one clique
// or the list of peers is empty
func IsSyncedWithAtLeastOnePeer(peers []alephium.InterCliquePeerInfo) bool {
	atLeastOneSynced := false
	for _, peer := range peers {
		if peer.IsSynced {
			atLeastOneSynced = true
		}
	}
	return atLeastOneSynced || len(peers) == 0
}

// WaitUntilSyncedWithAtLeastOnePeer waits until the clique is connected to at least one clique
// or the context is done.
func WaitUntilSyncedWithAtLeastOnePeer(ctx context.Context, alephiumClient *alephium.APIClient, sleeptime time.Duration, log *logrus.Entry) (bool, error) {
	for isSynced := false; ; {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}
		var err error
		isSynced, err = IsSynced(ctx, alephiumClient, log)
		if err != nil {
			return false, err
		}
		if isSynced {
			return true, nil
		} else {
			log.Debugf("Not sync'ed yet, sleeping %s", sleeptime)
			time.Sleep(sleeptime)
		}
	}
}

// IsSynced checks if the clique is synced
func IsSynced(ctx context.Context, alephiumClient *alephium.APIClient, log *logrus.Entry) (bool, error) {
	peers, err := getInterCliquePeerInfo(ctx, alephiumClient, log)
	if err != nil {
		return false, err
	}
	isSynced := IsSyncedWithAtLeastOnePeer(peers)
	return isSynced, nil
}
