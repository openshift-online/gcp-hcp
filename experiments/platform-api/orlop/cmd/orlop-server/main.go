package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/stdr"
	"github.com/thetechnick/orlop/pkg/apiserver"
	"github.com/thetechnick/orlop/pkg/apiserver/optional"
)

func main() {
	var (
		address     string
		privatePort int
		publicPort  int
		corsOrigins string
		enablePublic bool
		enableRBAC  bool
		enableAuthn bool
	)

	flag.StringVar(&address, "address", "0.0.0.0", "address to bind to")
	flag.IntVar(&privatePort, "private-port", 8080, "port for private API")
	flag.IntVar(&publicPort, "public-port", 8081, "port for public API")
	flag.BoolVar(&enablePublic, "enable-public-api", true, "enable public API server")
	flag.BoolVar(&enableRBAC, "enable-rbac", false, "enable RBAC authorization middleware")
	flag.BoolVar(&enableAuthn, "enable-authentication", false, "enable ServiceAccount authentication middleware")
	flag.StringVar(&corsOrigins, "cors-origins", "*", "comma-separated list of allowed CORS origins")
	flag.Parse()

	logger := stdr.New(nil)

	origins := []string{}
	if corsOrigins != "" {
		origins = strings.Split(corsOrigins, ",")
		for i, origin := range origins {
			origins[i] = strings.TrimSpace(origin)
		}
	}

	privateScheme := getPrivateScheme()
	privateRegistry := apiserver.NewResourceRegistry(privateScheme, apiserver.WithLogger(logger))
	for _, res := range getPrivateResources() {
		if err := privateRegistry.Register(res); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to register private resource: %v\n", err)
			os.Exit(1)
		}
	}

	var middleware []func(http.Handler) http.Handler

	if enableAuthn {
		mw, err := optional.SetupAuthentication(privateRegistry, logger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to setup authentication: %v\n", err)
			os.Exit(1)
		}
		middleware = append(middleware, mw)
		logger.Info("ServiceAccount authentication enabled")
	}

	if enableRBAC {
		mw, err := optional.SetupRBAC(privateRegistry, logger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to setup RBAC: %v\n", err)
			os.Exit(1)
		}
		middleware = append(middleware, mw)
		logger.Info("RBAC authorization enabled")
	}

	opts := apiserver.Options{
		Address:     address,
		CORSOrigins: origins,
		Private: apiserver.PrivateAPIOptions{
			Port:       privatePort,
			Registry:   privateRegistry,
			Scheme:     privateScheme,
			Middleware: middleware,
		},
		Public: apiserver.PublicAPIOptions{
			Enable:     enablePublic,
			Port:       publicPort,
			Resources:  getPublicResources(),
			Scheme:     getPublicScheme(),
			Middleware: middleware,
		},
		Logger: logger,
	}

	server, err := apiserver.New(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.Run(); err != nil {
			logger.Error(err, "Server error")
			os.Exit(1)
		}
	}()

	<-sigChan
	logger.Info("Shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error(err, "Server shutdown error")
		os.Exit(1)
	}

	logger.Info("Server stopped")
}
