package main

import (
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

	if env.WalletName == "" || env.WalletPassword == "" {
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

	minerWalletHandler, err := newMinerWalletHandler(alephiumClient, env.WalletName, env.WalletPassword,
		env.WalletMnemonic, env.WalletMnemonicPassphrase, env.PrintMnemonic, log)
	if err != nil {
		log.Fatalf("Got an error while creating the wallet handler. Err = %v", err)
	}

	wallet, err := minerWalletHandler.createAndUnlockWallet()
	if err != nil {
		log.Fatalf("Got an error while creating and/or unlocking the wallet %s. Err = %v", env.WalletName, err)
	}

	err = minerWalletHandler.updateMinersAddresses()
	if err != nil {
		log.Fatalf("Got an error while updating miners addresses. Err = %v", err)
	}

	log.Infof("Mining wallet %s is ready to be used, now waiting for the node to become in sync if needed.", wallet.Name)

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

		transferHandler, err := newTransferHandler(alephiumClient, wallet.Name, env.WalletPassword,
			env.TransferAddress, env.TransferMaxAmount, env.TransferFrequency, metrics, log)
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
