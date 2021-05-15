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
		log.Infof("Wallet %s not found, creating or restoring it now.")

		if env.WalletMnemonic != "" {
			restoredWallet, err := alephiumClient.RestoreWallet(env.WalletPassword, env.WalletMnemonic,
				env.WalletName, true, env.WalletMnemonicPassphrase)
			if err != nil {
				log.Fatalf("Got an error calling wallet restore endpoint %s. Err = %v", env.AlephiumEndpoint, err)
			}

			wallet, err = alephiumClient.GetWalletStatus(restoredWallet.Name)
			if err != nil {
				log.Fatalf("Got an error calling wallet status after a restore, wallet restoration probably didn't work... Err = %v", err)
			}
		} else {
			createdWallet, err := alephiumClient.CreateWallet(env.WalletName, env.WalletPassword,
				true, env.WalletMnemonicPassphrase)
			if err != nil {
				log.Fatalf("Got an error calling wallet create endpoint %s. Err = %v", env.AlephiumEndpoint, err)
			}
			if env.PrintMnemonic {
				log.Infof("[SENSITIVE] The mnemonic of the newly created wallet is %s. This mnemonic will never be printed again, make sure you write them down somewhere!",
					createdWallet.Mnemonic)
			}
			wallet, err = alephiumClient.GetWalletStatus(createdWallet.Name)
			if err != nil {
				log.Fatalf("Got an error calling wallet status after a create, wallet creation probably didn't work... Err = %v", err)
			}
		}
	}

	if wallet.Locked {
		ok, err := alephiumClient.UnlockWallet(wallet.Name, env.WalletPassword)
		if err != nil {
			log.Fatalf("Got an error calling wallet unlock. Err = %v", err)
		}
		if !ok {
			log.Fatalf("Unable to unlock the wallet %s, please make sure the provided password is correct and retry.", wallet.Name)
		}
	}

	minerAddresses, err := alephiumClient.GetMinersAddresses()
	if err != nil {
		log.Fatalf("Got an error calling miners addresses. Err = %v", err)
	}
	walletAddresses, err := alephiumClient.GetWalletAddresses(wallet.Name)
	if err != nil {
		log.Fatalf("Got an error calling wallet addresses. Err = %v", err)
	}
	if !hasSameAddresses(minerAddresses, walletAddresses) {
		err = alephiumClient.UpdateMinersAddresses(walletAddresses.Addresses)
		if err != nil {
			log.Fatalf("Got an error calling update miners addresses. Err = %v", err)
		}
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
			metrics.transferRun.Inc()

			wallet, err = alephiumClient.GetWalletStatus(wallet.Name)
			if err != nil {
				log.Fatalf("Got an error calling wallet status after a restore, wallet restoration probably didn't work... Err = %v", err)
			}
			if wallet.Locked {
				_, err := alephiumClient.UnlockWallet(wallet.Name, env.WalletPassword)
				if err != nil {
					log.Fatalf("Got an error calling wallet unlock. Err = %v", err)
				}
			}

			walletAddresses, err := alephiumClient.GetWalletAddresses(wallet.Name)
			if err != nil {
				log.Fatalf("Got an error calling wallet addresses. Err = %v", err)
			}
			walletBalances, err := alephiumClient.GetWalletBalances(wallet.Name)
			if err != nil {
				log.Fatalf("Got an error calling wallet balances. Err = %v", err)
			}

			amount := getAmount(walletAddresses.ActiveAddress, walletBalances.Balances)
			if amount != "" {
				roundAmount := roundAmount(amount, env.TransferMaxAmount)
				log.Debugf("%s, %s, %s", walletAddresses.ActiveAddress, amount, roundAmount)
				if roundAmount != "" {
					tx, err := alephiumClient.Transfer(wallet.Name, env.TransferAddress, roundAmount)
					if err != nil {
						log.Fatalf("Got an error calling transfer. Err = %v", err)
					}
					log.Debugf("New tx %s,%d->%d of %s from %s to %s", tx.TransactionId, tx.FromGroup, tx.ToGroup,
						amount, walletAddresses.ActiveAddress, env.TransferAddress)
				}
			}
			for _, address := range getNonActiveAddresses(walletAddresses) {

				ok, err := alephiumClient.ChangeActiveAddress(wallet.Name, address)
				if err != nil {
					log.Fatalf("Got an error calling change active address. Err = %v", err)
				}
				if !ok {
					log.Warnf("Got a false while calling change active address. Not sure what this means yet...")
				}
				amount := getAmount(address, walletBalances.Balances)
				if amount != "" {
					roundAmount := roundAmount(amount, env.TransferMaxAmount)
					log.Debugf("%s, %s, %s", walletAddresses.ActiveAddress, amount, roundAmount)
					if roundAmount != "" {
						tx, err := alephiumClient.Transfer(wallet.Name, env.TransferAddress, roundAmount)
						if err != nil {
							log.Fatalf("Got an error calling transfer. Err = %v", err)
						}
						log.Debugf("New tx %s,%d->%d of %s from %s to %s", tx.TransactionId, tx.FromGroup, tx.ToGroup,
							amount, address, env.TransferAddress)
					}
				}
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

var OneALP       =   "1000000000000000000"
var TenALP       =  "10000000000000000000"
var HundredALP   = "100000000000000000000"
var ThousandALP = "1000000000000000000000"

func roundAmount(amount string, txAmount string) string {

	balance, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return  ""
	}
	one, _ := new(big.Int).SetString(OneALP, 10)
	one = one.Neg(one)
	balance.Add(balance, one)
	if balance.Cmp(one) > 0 {
		alp, _ := new(big.Int).SetString(txAmount, 10)
		if balance.Cmp(alp) > 0 {
			return alp.Text(10)
		}
		return balance.Text(10)
	}
	return ""
}

func getNonActiveAddresses(walletAddresses alephium.WalletAddresses) []string {
	nonActiveAddresses := make([]string, 0, len(walletAddresses.Addresses) - 1)
	for _, wa := range walletAddresses.Addresses {
		if wa != walletAddresses.ActiveAddress {
			nonActiveAddresses = append(nonActiveAddresses, wa)
		}
	}
	return nonActiveAddresses
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