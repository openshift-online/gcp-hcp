package postgres

import (
	"context"
	"database/sql"

	_ "github.com/lib/pq"
)

// ExamplePostgreSQLStorage demonstrates how to configure the API server to use PostgreSQL storage.
func ExamplePostgreSQLStorage() {
	// Open PostgreSQL connection
	db, err := sql.Open("postgres", "postgres://user:pass@localhost/mydb?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Create PostgreSQL storage factory
	_ = NewStorageFactory(StorageFactoryConfig{
		DB:         db,
		ConnString: "postgres://user:pass@localhost/mydb?sslmode=disable",
		Context:    context.Background(),
	})

	// Create server with PostgreSQL storage
	// Note: This example assumes you have privateScheme and resources defined
	/*
		import "github.com/thetechnick/orlop/pkg/apiserver"

		server, err := apiserver.New(apiserver.Options{
			Address: "0.0.0.0",
			Private: apiserver.PrivateAPIOptions{
				Port:      8080,
				Scheme:    privateScheme,
				Resources: privateResources,
			},
			Public: apiserver.PublicAPIOptions{
				Enable:    true,
				Port:      8081,
				Scheme:    publicScheme,
				Resources: publicResources,
			},
			StorageFactory: storageFactory,
		})
		if err != nil {
			panic(err)
		}

		if err := server.Run(); err != nil {
			panic(err)
		}
	*/
}

// ExampleMemoryStorage demonstrates the default in-memory storage (no configuration needed).
func ExampleMemoryStorage() {
	// Create server with default in-memory storage
	// Note: This example assumes you have privateScheme and resources defined
	/*
		server, err := apiserver.New(apiserver.Options{
			Address: "0.0.0.0",
			Private: apiserver.PrivateAPIOptions{
				Port:      8080,
				Scheme:    privateScheme,
				Resources: privateResources,
			},
			Public: apiserver.PublicAPIOptions{
				Enable:    true,
				Port:      8081,
				Scheme:    publicScheme,
				Resources: publicResources,
			},
			// StorageFactory not specified - uses in-memory by default
		})
		if err != nil {
			panic(err)
		}

		if err := server.Run(); err != nil {
			panic(err)
		}
	*/
}

// ExampleCustomStorage demonstrates how to create a custom storage factory.
func ExampleCustomStorage() {
	// Create custom storage factory
	/*
		customFactory := func(resourceType string, scheme *runtime.Scheme, gvk schema.GroupVersionKind) (storage.ResourceStore, error) {
			// Your custom storage implementation
			return myCustomStore, nil
		}

		server, err := apiserver.New(apiserver.Options{
			Address: "0.0.0.0",
			Private: apiserver.PrivateAPIOptions{
				Port:      8080,
				Scheme:    privateScheme,
				Resources: privateResources,
			},
			StorageFactory: customFactory,
		})
		if err != nil {
			panic(err)
		}

		if err := server.Run(); err != nil {
			panic(err)
		}
	*/
}
