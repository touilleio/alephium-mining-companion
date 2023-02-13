package main

import (
	"context"
	"flag"
	"fmt"
	alephium "github.com/alephium/go-sdk"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"github.com/sqooba/go-common/healthchecks"
	"github.com/sqooba/go-common/logging"
	"github.com/sqooba/go-common/version"
	"golang.org/x/sync/errgroup"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

	// errgroup will coordinate the many routines handling the API.
	cancellableCtx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(cancellableCtx)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	s := http.Server{Addr: fmt.Sprint(":", env.Port)}
	g.Go(s.ListenAndServe)

	alephiumConfig := alephium.NewConfiguration()
	alephiumConfig.Host = env.AlephiumEndpoint
	if log.Level >= logrus.TraceLevel {
		alephiumConfig.Debug = true
	}
	if env.AlephiumApiKey != "" {
		alephiumConfig.DefaultHeader["X-API-KEY"] = env.AlephiumApiKey
	}
	alephiumClient := alephium.NewAPIClient(alephiumConfig)
	if err != nil {
		log.WithError(err).Fatalf("Got an error instantiating alephium client on %s", env.AlephiumEndpoint)
	}

	miningHandler, err := newMiningHandler(alephiumClient, env.WalletName, env.WalletPassword,
		env.WalletMnemonic, env.WalletMnemonicPassphrase, env.PrintMnemonic, log)
	if err != nil {
		log.Fatalf("Got an error while creating the wallet handler. Err = %v", err)
	}

	wallet, err := miningHandler.createAndUnlockWallet(ctx, logrus.NewEntry(log))
	if err != nil {
		log.Fatalf("Got an error while creating and/or unlocking the wallet %s. Err = %v", env.WalletName, err)
	}

	err = miningHandler.updateMinersAddresses(ctx, logrus.NewEntry(log))
	if err != nil {
		log.Fatalf("Got an error while updating miners addresses. Err = %v", err)
	}

	minersAddressesReq := alephiumClient.MinersApi.GetMinersAddresses(ctx)
	minersAddresses, _, err := minersAddressesReq.Execute()
	if err != nil {
		log.WithError(err).Fatalf("Got an error calling miners addresses")
	}
	log.Infof("Mining wallet %s (with addresses %v) is ready to be used, now waiting for the node to become in sync if needed.",
		wallet.WalletName, minersAddresses.Addresses)

	err = miningHandler.waitForNodeInSync(ctx, logrus.NewEntry(log))
	if err != nil {
		log.Fatalf("Got an error while waiting for the node to be in sync with peers. Err = %v", err)
	}
	g.Go(func() error {
		return miningHandler.ensureMiningWalletAndNodeMining(ctx, logrus.NewEntry(log))
	})

	addressesToWatch := make([]string, 0, len(minersAddresses.Addresses)+1)
	for _, a := range minersAddresses.Addresses {
		addressesToWatch = append(addressesToWatch, a)
	}
	if env.TransferAddress != "" {
		addressesToWatch = append(addressesToWatch, env.TransferAddress)
	}
	addressBalanceStats, _ := newAddressBalanceStats(alephiumClient, addressesToWatch, metrics)
	g.Go(func() error { return addressBalanceStats.Stats(ctx) })

	if env.TransferAddress != "" {
		transferHandler, err := newTransferHandler(alephiumClient, wallet.WalletName, env.WalletPassword,
			env.WalletMnemonicPassphrase, env.TransferAddress, env.TransferMinAmount, env.TransferFrequency,
			env.ImmediateTransfer, metrics, log)
		if err != nil {
			log.WithError(err).Fatalf("Got an error while instanciating the transfer handler")
		}

		log.Infof("We will transfer to %s the mining reward every %s.", env.TransferAddress, env.TransferFrequency)

		g.Go(func() error {
			err := transferHandler.handle(ctx, logrus.NewEntry(log))
			if err != nil {
				log.WithError(err).Fatalf("Got an error while handling the transfer")
			}
			return err
		})
	} else {
		log.Infof("No transfer address configure, no problem, job is done.")
		cancel()
	}

	// Wait for any shutdown
	select {
	case <-signalChan:
		log.Info("Shutdown signal received, exiting...")
		cancel()
		break
	case <-ctx.Done():
		log.Info("Group context is done, exiting...")
		cancel()
		break
	}

	err = ctx.Err()
	if err != nil {
		log.WithError(err).Fatal("Got an error from the error group context")
	}

	log.Infof("All good, stopping now.")
}
