package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/sqooba/go-common/healthchecks"
	"github.com/sqooba/go-common/logging"
	"github.com/sqooba/go-common/version"
	"github.com/touilleio/alephium-go-client"
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
	AlephiumApiKey           string        `envconfig:"ALEPHIUM_API_KEY" default:""`
	WalletName               string        `envconfig:"WALLET_NAME" default:"mining-companion-wallet-1"`
	WalletPassword           string        `envconfig:"WALLET_PASSWORD" default:"Default-Password-1234"`
	WalletMnemonic           string        `envconfig:"WALLET_MNEMONIC" default:""`
	WalletMnemonicPassphrase string        `envconfig:"WALLET_MNEMONIC_PASSPHRASE" default:""`
	TransferMinAmount        string        `envconfig:"TRANSFER_MIN_AMOUNT" default:"20000000000000000000"`
	TransferAddress          string        `envconfig:"TRANSFER_ADDRESS" default:""`
	TransferFrequency        time.Duration `envconfig:"TRANSFER_FREQUENCY" default:"15m"`
	PrintMnemonic            bool          `envconfig:"PRINT_MNEMONIC" default:"false"`
	ImmediateTransfer        bool          `envconfig:"IMMEDIATE_TRANSFER" default:"false"`

	MetricsNamespace string `envconfig:"METRICS_NAMESPACE" default:"alephium"`
	MetricsSubsystem string `envconfig:"METRICS_SUBSYSTEM" default:"miningcompanion"`
	MetricsPath      string `envconfig:"METRICS_PATH" default:"/metrics"`
}

const (
	DefaultWalletPassword = "Default-Password-1234"
)

func main() {

	log.Println("Alephium-mining-companion application is initializing...")
	log.Printf("Version    : %s", version.Version)
	log.Printf("Commit     : %s", version.GitCommit)
	log.Printf("Build date : %s", version.BuildDate)
	log.Printf("OSarch     : %s", version.OsArch)

	rand.Seed(time.Now().UnixNano())

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env var: %s\n", err)
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

	if env.WalletName == "" || env.WalletPassword == "" {
		log.Fatalf("Some mandatory configuration parameters are missing. Please correct the config and retry.")
	}
	if env.WalletPassword == DefaultWalletPassword {
		log.Warnf("Your using the default password. This is not recommanded for production use.")
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

	alephiumClient, err := alephium.NewWithApiKey(env.AlephiumEndpoint, env.AlephiumApiKey, log)
	if err != nil {
		log.Fatalf("Got an error instantiating alephium client on %s. Err = %v", env.AlephiumEndpoint, err)
	}

	miningHandler, err := newMiningHandler(alephiumClient, env.WalletName, env.WalletPassword,
		env.WalletMnemonic, env.WalletMnemonicPassphrase, env.PrintMnemonic, log)
	if err != nil {
		log.Fatalf("Got an error while creating the wallet handler. Err = %v", err)
	}

	wallet, err := miningHandler.createAndUnlockWallet()
	if err != nil {
		log.Fatalf("Got an error while creating and/or unlocking the wallet %s. Err = %v", env.WalletName, err)
	}

	err = miningHandler.updateMinersAddresses()
	if err != nil {
		log.Fatalf("Got an error while updating miners addresses. Err = %v", err)
	}

	minerAddresses, err := alephiumClient.GetMinersAddresses()
	if err != nil {
		log.Fatalf("Got an error calling miners addresses. Err = %v", err)
	}
	log.Infof("Mining wallet %s (with addresses %v) is ready to be used, now waiting for the node to become in sync if needed.",
		wallet.Name, minerAddresses.Addresses)

	err = miningHandler.waitForNodeInSync()
	if err != nil {
		log.Fatalf("Got an error while waiting for the node to be in sync with peers. Err = %v", err)
	}
	go miningHandler.ensureMiningWalletAndNodeMining()

	addressesToWatch := make([]string, 0, len(minerAddresses.Addresses) + 1)
	for _, a := range minerAddresses.Addresses {
		addressesToWatch = append(addressesToWatch, a)
	}
	if env.TransferAddress != "" {
		addressesToWatch = append(addressesToWatch, env.TransferAddress)
	}
	addressBalanceStats, _ := newAddressBalanceStats(alephiumClient, addressesToWatch, metrics)
	go addressBalanceStats.Stats(context.Background())

	if env.TransferAddress != "" {
		transferHandler, err := newTransferHandler(alephiumClient, wallet.Name, env.WalletPassword,
			env.WalletMnemonicPassphrase, env.TransferAddress, env.TransferMinAmount, env.TransferFrequency,
			env.ImmediateTransfer, metrics, log)
		if err != nil {
			log.Fatalf("Got an error while instanciating the transfer handler. Err = %v", err)
		}

		log.Infof("We will transfer to %s the mining reward every %s.", env.TransferAddress, env.TransferFrequency)

		err = transferHandler.handle()
		if err != nil {
			log.Fatalf("Got an error while handling the transfer. Err = %v", err)
		}
	} else {
		log.Infof("No transfer address configure, no problem, job is done.")
	}

	log.Infof("All good, stopping now.")
}
