package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	gogopubsub "cloud.google.com/go/pubsub/v2"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"k8s.io/client-go/util/workqueue"

	hcadapter "github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/adapters/hc"
	nodepooladapter "github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/adapters/nodepool"
	nodepoolvrresolution "github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/adapters/nodepoolvrresolution"
	placementadapter "github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/adapters/placement"
	versionresolution "github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/adapters/versionresolution"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/hyperfleetapi"
	pubsubpkg "github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/pubsub"
	workerqueue "github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/workqueue"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/maestroclient"
	maestrotransport "github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/transport/maestro"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
)

// rootFlags holds values bound to the root persistent flags.
type rootFlags struct {
	logLevel   string
	logFormat  string
	apiURL     string
	apiVersion string
	workers    int
	resync     time.Duration
}

// pubsubFlags holds the common pub/sub flags shared by all subcommands.
type pubsubFlags struct {
	pubsubProject string
	subscription  string
}

// maestroFlags holds Maestro-related flags shared by hc and nodepool subcommands.
type maestroFlags struct {
	grpcAddr  string
	httpAddr  string
	sourceID  string
	clientID  string
	insecure  bool
}

// envOr returns the value of the environment variable named by key, or
// fallback if the variable is unset or empty.
func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func main() {
	rf := &rootFlags{}

	root := &cobra.Command{
		Use:   "hyperfleet-adapters-go",
		Short: "HyperFleet Go adapters",
		// PersistentPreRun applies environment-variable overrides to root flags
		// before any subcommand runs.
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if v := envOr("LOG_LEVEL", ""); v != "" && !cmd.Flags().Changed("log-level") {
				rf.logLevel = v
			}
			if v := envOr("LOG_FORMAT", ""); v != "" && !cmd.Flags().Changed("log-format") {
				rf.logFormat = v
			}
			if v := envOr("HYPERFLEET_API_URL", ""); v != "" && !cmd.Flags().Changed("api-url") {
				rf.apiURL = v
			}
			if v := envOr("API_VERSION", ""); v != "" && !cmd.Flags().Changed("api-version") {
				rf.apiVersion = v
			}
		},
	}

	// Root persistent flags.
	root.PersistentFlags().StringVar(&rf.logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	root.PersistentFlags().StringVar(&rf.logFormat, "log-format", "json", "Log format (json, text)")
	root.PersistentFlags().StringVar(&rf.apiURL, "api-url", "http://hyperfleet-api:8000", "HyperFleet API base URL [$HYPERFLEET_API_URL]")
	root.PersistentFlags().StringVar(&rf.apiVersion, "api-version", "v1", "HyperFleet API version")
	root.PersistentFlags().IntVar(&rf.workers, "workers", 10, "Number of worker goroutines")
	root.PersistentFlags().DurationVar(&rf.resync, "resync", 5*time.Minute, "Resync period")

	root.AddCommand(
		newVersionResolutionCmd(rf),
		newNodepoolVRCmd(rf),
		newPlacementCmd(rf),
		newHCCmd(rf),
		newNodepoolCmd(rf),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newLogger creates a logger from root flags.
func newLogger(rf *rootFlags, component string) (logger.Logger, error) {
	return logger.NewLogger(logger.Config{
		Level:     rf.logLevel,
		Format:    rf.logFormat,
		Output:    "stdout",
		Component: component,
	})
}

// newZapSugared creates a zap sugared logger for pubsub/workqueue internals.
func newZapSugared() *zap.SugaredLogger {
	zapLogger, _ := zap.NewProduction()
	return zapLogger.Sugar()
}

// ─── version-resolution ──────────────────────────────────────────────────────

func newVersionResolutionCmd(rf *rootFlags) *cobra.Command {
	psf := &pubsubFlags{}
	var cincinnatiURL, arch string

	cmd := &cobra.Command{
		Use:   "version-resolution",
		Short: "Run the version-resolution adapter",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Apply env overrides for adapter-specific flags.
			if v := envOr("PUBSUB_PROJECT", ""); v != "" && !cmd.Flags().Changed("pubsub-project") {
				psf.pubsubProject = v
			}

			ctx := cmd.Context()

			log, err := newLogger(rf, "version-resolution-adapter")
			if err != nil {
				return fmt.Errorf("create logger: %w", err)
			}

			hfClient := hyperfleetapi.New(rf.apiURL, rf.apiVersion, log)
			cinClient := versionresolution.NewCincinnatiClient(cincinnatiURL, arch)
			rec := versionresolution.NewReconciler(hfClient, cinClient, log)

			q := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
			zapSugared := newZapSugared()

			psClient, err := gogopubsub.NewClient(ctx, psf.pubsubProject)
			if err != nil {
				return fmt.Errorf("create pubsub client: %w", err)
			}
			sub := psClient.Subscriber(psf.subscription)
			subscriber := pubsubpkg.New(sub, q, zapSugared)

			go subscriber.Run(ctx)
			workerqueue.Run(ctx, rf.workers, q, rec.Reconcile, zapSugared)
			return nil
		},
	}

	cmd.Flags().StringVar(&psf.pubsubProject, "pubsub-project", "", "GCP project for Pub/Sub [$PUBSUB_PROJECT]")
	cmd.Flags().StringVar(&psf.subscription, "subscription", "hyperfleet-cluster-events-vr-adapter", "Pub/Sub subscription name")
	cmd.Flags().StringVar(&cincinnatiURL, "cincinnati-url", "https://api.openshift.com/api/upgrades_info/v1/graph", "Cincinnati API URL")
	cmd.Flags().StringVar(&arch, "arch", "amd64", "CPU architecture for Cincinnati query")

	return cmd
}

// ─── nodepool-vr ─────────────────────────────────────────────────────────────

func newNodepoolVRCmd(rf *rootFlags) *cobra.Command {
	psf := &pubsubFlags{}
	var cincinnatiURL, arch string

	cmd := &cobra.Command{
		Use:   "nodepool-vr",
		Short: "Run the nodepool version-resolution adapter",
		RunE: func(cmd *cobra.Command, args []string) error {
			if v := envOr("PUBSUB_PROJECT", ""); v != "" && !cmd.Flags().Changed("pubsub-project") {
				psf.pubsubProject = v
			}

			ctx := cmd.Context()

			log, err := newLogger(rf, "nodepool-vr-adapter")
			if err != nil {
				return fmt.Errorf("create logger: %w", err)
			}

			hfClient := hyperfleetapi.New(rf.apiURL, rf.apiVersion, log)
			cinClient := versionresolution.NewCincinnatiClient(cincinnatiURL, arch)
			rec := nodepoolvrresolution.NewReconciler(hfClient, cinClient, log)

			q := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
			zapSugared := newZapSugared()

			psClient, err := gogopubsub.NewClient(ctx, psf.pubsubProject)
			if err != nil {
				return fmt.Errorf("create pubsub client: %w", err)
			}
			sub := psClient.Subscriber(psf.subscription)
			subscriber := pubsubpkg.New(sub, q, zapSugared)

			go subscriber.Run(ctx)
			workerqueue.Run(ctx, rf.workers, q, rec.Reconcile, zapSugared)
			return nil
		},
	}

	cmd.Flags().StringVar(&psf.pubsubProject, "pubsub-project", "", "GCP project for Pub/Sub [$PUBSUB_PROJECT]")
	cmd.Flags().StringVar(&psf.subscription, "subscription", "hyperfleet-nodepool-events-nodepool-vr-adapter", "Pub/Sub subscription name")
	cmd.Flags().StringVar(&cincinnatiURL, "cincinnati-url", "https://api.openshift.com/api/upgrades_info/v1/graph", "Cincinnati API URL")
	cmd.Flags().StringVar(&arch, "arch", "amd64", "CPU architecture for Cincinnati query")

	return cmd
}

// ─── placement ───────────────────────────────────────────────────────────────

func newPlacementCmd(rf *rootFlags) *cobra.Command {
	psf := &pubsubFlags{}
	var candidateNames, baseDomains []string
	var smProject, maestroHTTPAddr string

	cmd := &cobra.Command{
		Use:   "placement",
		Short: "Run the placement adapter",
		RunE: func(cmd *cobra.Command, args []string) error {
			if v := envOr("PUBSUB_PROJECT", ""); v != "" && !cmd.Flags().Changed("pubsub-project") {
				psf.pubsubProject = v
			}
			if v := envOr("SECRETMANAGER_PROJECT", ""); v != "" && !cmd.Flags().Changed("secretmanager-project") {
				smProject = v
			}
			if v := envOr("MAESTRO_HTTP_ADDR", ""); v != "" && !cmd.Flags().Changed("maestro-http-addr") {
				maestroHTTPAddr = v
			}

			ctx := cmd.Context()

			log, err := newLogger(rf, "placement-adapter")
			if err != nil {
				return fmt.Errorf("create logger: %w", err)
			}

			hfClient := hyperfleetapi.New(rf.apiURL, rf.apiVersion, log)

			var selector placementadapter.Selector
			var candidates []placementadapter.Candidate

			if smProject != "" {
				// Dynamic mode: discover MCs and DNS zones from Secret Manager + Maestro.
				smClient, err := secretmanager.NewClient(ctx)
				if err != nil {
					return fmt.Errorf("create secret manager client: %w", err)
				}
				defer smClient.Close() //nolint:errcheck
				selector = placementadapter.NewDynamicSelector(smClient, smProject, maestroHTTPAddr)
			} else {
				// Static mode: use explicitly provided --candidates / --base-domains.
				candidates = make([]placementadapter.Candidate, 0, len(candidateNames))
				for i, name := range candidateNames {
					c := placementadapter.Candidate{Name: name}
					if i < len(baseDomains) {
						c.BaseDomains = []string{baseDomains[i]}
					}
					candidates = append(candidates, c)
				}
				selector = placementadapter.NewRoundRobinSelector()
			}

			rec := placementadapter.NewReconciler(hfClient, selector, candidates, log)

			q := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
			zapSugared := newZapSugared()

			psClient, err := gogopubsub.NewClient(ctx, psf.pubsubProject)
			if err != nil {
				return fmt.Errorf("create pubsub client: %w", err)
			}
			sub := psClient.Subscriber(psf.subscription)
			subscriber := pubsubpkg.New(sub, q, zapSugared)

			go subscriber.Run(ctx)
			workerqueue.Run(ctx, rf.workers, q, rec.Reconcile, zapSugared)
			return nil
		},
	}

	cmd.Flags().StringVar(&psf.pubsubProject, "pubsub-project", "", "GCP project for Pub/Sub [$PUBSUB_PROJECT]")
	cmd.Flags().StringVar(&psf.subscription, "subscription", "hyperfleet-cluster-events-placement-adapter", "Pub/Sub subscription name")
	cmd.Flags().StringSliceVar(&candidateNames, "candidates", nil, "MC names (comma-separated); ignored when --secretmanager-project is set")
	cmd.Flags().StringSliceVar(&baseDomains, "base-domains", nil, "Base domains per MC, paired with --candidates; ignored when --secretmanager-project is set")
	cmd.Flags().StringVar(&smProject, "secretmanager-project", "", "GCP project for Secret Manager MC/DNS discovery [$SECRETMANAGER_PROJECT]; enables dynamic selector")
	cmd.Flags().StringVar(&maestroHTTPAddr, "maestro-http-addr", "http://maestro.hyperfleet.svc.cluster.local:8000", "Maestro HTTP API URL for consumer discovery [$MAESTRO_HTTP_ADDR]")

	return cmd
}

// ─── hc ──────────────────────────────────────────────────────────────────────

func newHCCmd(rf *rootFlags) *cobra.Command {
	psf := &pubsubFlags{}
	mf := &maestroFlags{}

	cmd := &cobra.Command{
		Use:   "hc",
		Short: "Run the hosted-cluster (hc) adapter",
		RunE: func(cmd *cobra.Command, args []string) error {
			if v := envOr("PUBSUB_PROJECT", ""); v != "" && !cmd.Flags().Changed("pubsub-project") {
				psf.pubsubProject = v
			}

			ctx := cmd.Context()

			log, err := newLogger(rf, "hc-adapter")
			if err != nil {
				return fmt.Errorf("create logger: %w", err)
			}

			mwc, err := maestroclient.NewMaestroClient(ctx, &maestroclient.Config{
				MaestroServerAddr: mf.httpAddr,
				GRPCServerAddr:    mf.grpcAddr,
				SourceID:          mf.sourceID,
				Insecure:          mf.insecure,
			}, log)
			if err != nil {
				return fmt.Errorf("create maestro client: %w", err)
			}
			defer mwc.Close() //nolint:errcheck

			transport := maestrotransport.New(mwc, mf.sourceID, log)

			hfClient := hyperfleetapi.New(rf.apiURL, rf.apiVersion, log)
			rec := hcadapter.New(hfClient, transport, log)

			q := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
			zapSugared := newZapSugared()

			psClient, err := gogopubsub.NewClient(ctx, psf.pubsubProject)
			if err != nil {
				return fmt.Errorf("create pubsub client: %w", err)
			}
			sub := psClient.Subscriber(psf.subscription)
			subscriber := pubsubpkg.New(sub, q, zapSugared)

			go subscriber.Run(ctx)
			workerqueue.Run(ctx, rf.workers, q, rec.Reconcile, zapSugared)
			return nil
		},
	}

	cmd.Flags().StringVar(&psf.pubsubProject, "pubsub-project", "", "GCP project for Pub/Sub [$PUBSUB_PROJECT]")
	cmd.Flags().StringVar(&psf.subscription, "subscription", "hyperfleet-cluster-events-hc-adapter", "Pub/Sub subscription name")
	cmd.Flags().StringVar(&mf.grpcAddr, "maestro-grpc-addr", "maestro-grpc.hyperfleet.svc.cluster.local:8090", "Maestro gRPC server address")
	cmd.Flags().StringVar(&mf.httpAddr, "maestro-http-addr", "http://maestro.hyperfleet.svc.cluster.local:8000", "Maestro HTTP API server address")
	cmd.Flags().StringVar(&mf.sourceID, "maestro-source-id", "hc-adapter", "Maestro source ID")
	cmd.Flags().StringVar(&mf.clientID, "maestro-client-id", "hc-adapter-client", "Maestro client ID")
	cmd.Flags().BoolVar(&mf.insecure, "maestro-insecure", true, "Disable TLS verification for Maestro connections")

	return cmd
}

// ─── nodepool ────────────────────────────────────────────────────────────────

func newNodepoolCmd(rf *rootFlags) *cobra.Command {
	psf := &pubsubFlags{}
	mf := &maestroFlags{}

	cmd := &cobra.Command{
		Use:   "nodepool",
		Short: "Run the nodepool adapter",
		RunE: func(cmd *cobra.Command, args []string) error {
			if v := envOr("PUBSUB_PROJECT", ""); v != "" && !cmd.Flags().Changed("pubsub-project") {
				psf.pubsubProject = v
			}

			ctx := cmd.Context()

			log, err := newLogger(rf, "nodepool-adapter")
			if err != nil {
				return fmt.Errorf("create logger: %w", err)
			}

			mwc, err := maestroclient.NewMaestroClient(ctx, &maestroclient.Config{
				MaestroServerAddr: mf.httpAddr,
				GRPCServerAddr:    mf.grpcAddr,
				SourceID:          mf.sourceID,
				Insecure:          mf.insecure,
			}, log)
			if err != nil {
				return fmt.Errorf("create maestro client: %w", err)
			}
			defer mwc.Close() //nolint:errcheck

			transport := maestrotransport.New(mwc, mf.sourceID, log)

			hfClient := hyperfleetapi.New(rf.apiURL, rf.apiVersion, log)
			rec := nodepooladapter.New(hfClient, transport, log)

			q := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
			zapSugared := newZapSugared()

			psClient, err := gogopubsub.NewClient(ctx, psf.pubsubProject)
			if err != nil {
				return fmt.Errorf("create pubsub client: %w", err)
			}
			sub := psClient.Subscriber(psf.subscription)
			subscriber := pubsubpkg.New(sub, q, zapSugared)

			go subscriber.Run(ctx)
			workerqueue.Run(ctx, rf.workers, q, rec.Reconcile, zapSugared)
			return nil
		},
	}

	cmd.Flags().StringVar(&psf.pubsubProject, "pubsub-project", "", "GCP project for Pub/Sub [$PUBSUB_PROJECT]")
	cmd.Flags().StringVar(&psf.subscription, "subscription", "hyperfleet-nodepool-events-nodepool-adapter", "Pub/Sub subscription name")
	cmd.Flags().StringVar(&mf.grpcAddr, "maestro-grpc-addr", "maestro-grpc.hyperfleet.svc.cluster.local:8090", "Maestro gRPC server address")
	cmd.Flags().StringVar(&mf.httpAddr, "maestro-http-addr", "http://maestro.hyperfleet.svc.cluster.local:8000", "Maestro HTTP API server address")
	cmd.Flags().StringVar(&mf.sourceID, "maestro-source-id", "nodepool-adapter", "Maestro source ID")
	cmd.Flags().StringVar(&mf.clientID, "maestro-client-id", "nodepool-adapter-client", "Maestro client ID")
	cmd.Flags().BoolVar(&mf.insecure, "maestro-insecure", true, "Disable TLS verification for Maestro connections")

	return cmd
}
