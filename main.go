package main

import (
	"flag"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/sqooba/go-common/healthchecks"
	"github.com/sqooba/go-common/logging"
	"github.com/sqooba/go-common/version"
	"github.com/touilleio/alephium-go-client"
	"math/big"
	"math/rand"
	"net/http"
	"time"
)

var (
	healthCheck = flag.Bool("health-check", false, "Run health-check")
	setLogLevel = flag.String("set-log-level", "", "Change log level. Possible values are trace,debug,info,warn,error,fatal,panic")
	log         = logging.NewLogger()
)

type envConfig struct {
	Port     string `envconfig:"PORT" default:"8080"`
	LogLevel string `envconfig:"LOG_LEVEL" default:"debug"`

	AlephiumEndpoint         string        `envconfig:"ALEPHIUM_ENDPOINT" default:"http://alephium:12973"`
	WalletName               string        `envconfig:"WALLET_NAME" default:""`
	WalletPassword           string        `envconfig:"WALLET_PASSWORD" default:""`
	WalletMnemonic           string        `envconfig:"WALLET_MNEMONIC" default:""`
	WalletMnemonicPassphrase string        `envconfig:"WALLET_MNEMONIC_PASSPHRASE" default:""`
	TransferMaxAmount        string        `envconfig:"TRANSFER_MAX_AMOUNT" default:"50000000000000000000"`
	TransferAddress          string        `envconfig:"TRANSFER_ADDRESS" default:""`
	TransferFrequency        time.Duration `envconfig:"TRANSFER_FREQUENCY" default:"15m"`
	PrintMnemonic            bool          `envconfig:"PRINT_MNEMONIC" default:"true"`

	MetricsNamespace string `envconfig:"METRICS_NAMESPACE" default:"alephium"`
	MetricsSubsystem string `envconfig:"METRICS_SUBSYSTEM" default:"miningsidecar"`
	MetricsPath      string `envconfig:"METRICS_PATH" default:"/metrics"`
}

func main() {

	log.Println("alephium-mining-sidecar application is initializing...")
	log.Printf("Version    : %s", version.Version)
	log.Printf("Commit     : %s", version.GitCommit)
	log.Printf("Build date : %s", version.BuildDate)
	log.Printf("OSarch     : %s", version.OsArch)

	rand.Seed(time.Now().UnixNano())

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Printf("[ERROR] Failed to process env var: %s\n", err)
		return
	}

	flag.Parse()

	err := logging.SetLogLevel(log, env.LogLevel)
	if err != nil {
		log.Fatalf("Logging level %s do not seem to be right. Err = %v", env.LogLevel, err)
	}

	// Running health check (so that it can be the same binary in the containers
	if *healthCheck {
		healthchecks.RunHealthCheckAndExit(env.Port)
	}
	if *setLogLevel != "" {
		logging.SetRemoteLogLevelAndExit(log, env.Port, *setLogLevel)
	}

	if env.WalletName == "" || env.WalletPassword == "" ||
		env.TransferAddress == "" {
		log.Fatalf("Some mandatory configuration parameters are missing. Please correct the config and retry.")
	}

	// Register health checks and metrics
	initHealthChecks(env, http.DefaultServeMux)
	metrics := initPrometheus(env, http.DefaultServeMux)

	// Special endpoint to change the verbosity at runtime, i.e. curl -X PUT --data debug ...
	logging.InitVerbosityHandler(log, http.DefaultServeMux)

	s := http.Server{Addr: fmt.Sprint(":", env.Port)}
	go func() {
		log.Fatal(s.ListenAndServe())
	}()

	alephiumClient, err := alephium.New(env.AlephiumEndpoint, log)
	if err != nil {
		log.Fatalf("Got an error instantiating alephium client on %s. Err = %v", env.AlephiumEndpoint, err)
	}

	var wallet alephium.WalletInfo
	var walletFound bool
	wallets, err := alephiumClient.GetWallets()
	if err != nil {
		log.Fatalf("Got an error calling wallets endpoint %s. Err = %v", env.AlephiumEndpoint, err)
	}
	for _, w := range wallets {
		if w.Name == env.WalletName {
			wallet = w
			walletFound = true
			break
		}
	}
	if !walletFound {
		wallet, err = createWallet(alephiumClient, env.WalletName, env.WalletPassword,
			env.WalletMnemonic, env.WalletMnemonicPassphrase, env.PrintMnemonic)
		if err != nil {
			log.Fatalf("Got an error while creating the wallet. Err = %v", err)
		}
	}

	if wallet.Locked {
		ok, err := alephiumClient.UnlockWallet(wallet.Name, env.WalletPassword)
		if err != nil {
			log.Fatalf("Got an error while unlocking the wallet %s. Err = %v", wallet.Name, err)
		}
		if !ok {
			log.Fatalf("Unable to unlock the wallet %s, please make sure the provided password is correct and retry.", wallet.Name)
		}
	}

	err = updateMinersAddresses(alephiumClient, wallet.Name)
	if err != nil {
		log.Fatalf("Got an error while updating miners addresses. Err = %v", err)
	}

	log.Infof("Mining wallet %s is ready to be used, now waiting for the node to become in sync.", wallet.Name)

	err = alephiumClient.WaitUntilSyncedWithAtLeastOnePeer()
	if err != nil {
		log.Fatalf("Got an error waiting for the node to be in sync with peers. Err = %v", err)
	}

	log.Infof("Node %s is ready to mine, starting the mining now.", env.AlephiumEndpoint)

	_, err = alephiumClient.StartMining()
	if err != nil {
		log.Fatalf("Got an error starting the mining. Err = %v", err)
	}

	if env.TransferAddress != "" {

		log.Infof("We will transfer to %s the mining reward every %s.", env.TransferAddress, env.TransferFrequency)

		for range time.Tick(env.TransferFrequency) {
			err = transfer(alephiumClient, wallet.Name, env.WalletPassword,
				env.TransferAddress, env.TransferMaxAmount, metrics)
			if err != nil {
				log.Fatalf("Got an error while transferring some amount #2. Err = %v", err)
			}
		}
	}

	log.Infof("All good, stopping now.")
}

func getAmount(address string, balances []alephium.AddressBalance) string {
	for _, b := range balances {
		if b.Address == address {
			return b.Balance
		}
	}
	return ""
}

var OneALP      =   "1000000000000000000"
var TwoALP      =   "2000000000000000000"
var TenALP      =  "10000000000000000000"
var HundredALP  = "100000000000000000000"
var ThousandALP = "1000000000000000000000"
var two         = big.NewInt(2)

func roundAmount(amount string, txAmount string) string {
	balance, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return  ""
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

func createWallet(alephiumClient *alephium.AlephiumClient, walletName string, walletPassword string,
	walletMnemonic string, walletMnemonicPassphrase string, printMnemonic bool) (alephium.WalletInfo, error) {

	log.Infof("Wallet %s not found, creating or restoring it now.", walletName)

	var wallet alephium.WalletInfo

	if walletMnemonic != "" {
		restoredWallet, err := alephiumClient.RestoreWallet(walletPassword, walletMnemonic,
			walletName, true, walletMnemonicPassphrase)
		if err != nil {
			log.Debugf("Got an error calling wallet restore endpoint %v. Err = %v", alephiumClient, err)
			return wallet, err
		}

		wallet, err = alephiumClient.GetWalletStatus(restoredWallet.Name)
		if err != nil {
			log.Debugf("Got an error calling wallet status after a restore, wallet restoration probably didn't work... Err = %v", err)
			return wallet, err
		}
	} else {
		createdWallet, err := alephiumClient.CreateWallet(walletName, walletPassword,
			true, walletMnemonicPassphrase)
		if err != nil {
			log.Debugf("Got an error calling wallet create endpoint %v. Err = %v", alephiumClient, err)
			return wallet, err
		}
		if printMnemonic {
			log.Infof("[SENSITIVE] The mnemonic of the newly created wallet is %s. This mnemonic will never be printed again, make sure you write them down somewhere!",
				createdWallet.Mnemonic)
		}
		wallet, err = alephiumClient.GetWalletStatus(createdWallet.Name)
		if err != nil {
			log.Debugf("Got an error calling wallet status after a create, wallet creation probably didn't work... Err = %v", err)
			return wallet, err
		}
	}
	return wallet, nil
}

func updateMinersAddresses(alephiumClient *alephium.AlephiumClient, walletName string) error {
	minerAddresses, err := alephiumClient.GetMinersAddresses()
	if err != nil {
		log.Debugf("Got an error calling miners addresses. Err = %v", err)
		return err
	}
	walletAddresses, err := alephiumClient.GetWalletAddresses(walletName)
	if err != nil {
		log.Debugf("Got an error calling wallet addresses. Err = %v", err)
		return err
	}
	if !hasSameAddresses(minerAddresses, walletAddresses) {
		err = alephiumClient.UpdateMinersAddresses(walletAddresses.Addresses)
		if err != nil {
			log.Debugf("Got an error calling update miners addresses. Err = %v", err)
			return err
		}
	}
	return nil
}

func transfer(alephiumClient *alephium.AlephiumClient, walletName string, walletPassword string,
	transferAddress string, transferMaxAmount string, metrics *metrics) error {

	metrics.transferRun.Inc()

	wallet, err := alephiumClient.GetWalletStatus(walletName)
	if err != nil {
		log.Debugf("Got an error calling wallet status after a restore, wallet restoration probably didn't work... Err = %v", err)
		return err
	}
	if wallet.Locked {
		_, err := alephiumClient.UnlockWallet(wallet.Name, walletPassword)
		if err != nil {
			log.Debugf("Got an error calling wallet unlock. Err = %v", err)
			return err
		}
	}

	walletAddresses, err := alephiumClient.GetWalletAddresses(wallet.Name)
	if err != nil {
		log.Debugf("Got an error while getting wallet addresses. Err = %v", err)
		return err
	}
	walletBalances, err := alephiumClient.GetWalletBalances(wallet.Name)
	if err != nil {
		log.Debugf("Got an error while getting wallet balances. Err = %v", err)
		return err
	}

	for _, address := range getAddressesInRandomOrder(walletAddresses) {

		ok, err := alephiumClient.ChangeActiveAddress(wallet.Name, address)
		if err != nil {
			log.Debugf("Got an error calling change active address. Err = %v", err)
			return err
		}
		if !ok {
			log.Warnf("Got a false while calling change active address. Not sure what this means yet...")
		}
		amount := getAmount(address, walletBalances.Balances)
		log.Debugf("address %s, amount %s", address, amount)
		if amount != "" {
			roundAmount := roundAmount(amount, transferMaxAmount)
			log.Debugf("%s, %s, %s", address, amount, roundAmount)
			if roundAmount != "" {
				tx, err := alephiumClient.Transfer(wallet.Name, transferAddress, roundAmount)
				if err != nil {
					log.Debugf("Got an error calling transfer. Err = %v", err)
					return err
				}
				log.Debugf("New tx %s,%d->%d of %s from %s to %s", tx.TransactionId, tx.FromGroup, tx.ToGroup,
					roundAmount, address, transferAddress)
			}
		}
	}
	return nil
}