package main

import (
	"database/sql"
	"fmt"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalTestContainer struct {
	appName            string
	dbName             string
	appcontainer       *dockertest.Resource
	dbcontainer        *dockertest.Resource
	pool               *dockertest.Pool
	network            string
	appport            string
	dbmigratecontainer *dockertest.Resource
}

func CreateLocalTestContainer() (*LocalTestContainer, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
		return nil, err
	}
	// Create network
	networkName := "app-datastore"
	network, err := createNetwork(networkName, err, pool)

	// Create Postgres container
	dbresource := createPostgresDB(err, pool, network)
	log.Println(dbresource.Container.Name)

	port := "5432"
	name := strings.Trim(dbresource.Container.Name, "/")
	databaseUrl := fmt.Sprintf("postgres://user_name:secret@%s:%s/dbname?sslmode=disable", name, port)
	log.Println("Connecting to database on url: ", databaseUrl)

	testDBConnectivity(pool, dbresource)

	// Copy migration files to a temporary directory
	tempDir, err := os.MkdirTemp("", "migrations")
	if err != nil {
		log.Fatalf("Could not create temp dir: %s", err)
	}
	defer os.RemoveAll(tempDir)
	// Assuming you have the migrations in the ./migrations directory
	copyDir("./db/migrations", tempDir)

	// Create migration container
	dbmigrate := createMigration(err, pool, network, databaseUrl, tempDir, dbresource)

	log.Println(dbmigrate.Container.Name)

	// Create application container
	appresource := createAppContainer(err, pool, databaseUrl, network)

	appport := appresource.GetPort("8000/tcp")

	return &LocalTestContainer{
		appName:            appresource.Container.Name,
		dbName:             dbresource.Container.Name,
		appcontainer:       appresource,
		dbcontainer:        dbresource,
		dbmigratecontainer: dbmigrate,
		appport:            appport,
		pool:               pool,
		network:            network.ID,
	}, nil

}

func testDBConnectivity(pool *dockertest.Pool, dbresource *dockertest.Resource) {
	// Exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	if err := pool.Retry(func() error {
		var err error
		db, err := sql.Open("postgres", fmt.Sprintf("postgres://user_name:secret@localhost:%s/dbname?sslmode=disable", dbresource.GetPort("5432/tcp")))
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}
}

func createNetwork(networkName string, err error, pool *dockertest.Pool) (*docker.Network, error) {
	// Check if network exists
	network, err := findNetwork(networkName, pool)
	if err != nil {
		log.Fatalf("Could not list networks: %s", err)
	}
	if network == nil {
		network, err = pool.Client.CreateNetwork(docker.CreateNetworkOptions{
			Name:           networkName,
			Driver:         "bridge",
			CheckDuplicate: true,
		})
		if err != nil {
			log.Fatalf("Could not create network: %s", err)
		}
	}
	return network, err
}

func createAppContainer(err error, pool *dockertest.Pool, databaseUrl string, network *docker.Network) *dockertest.Resource {
	targetArch := "amd64" // or "arm64", depending on your needs
	appresource, err := pool.BuildAndRunWithBuildOptions(&dockertest.BuildOptions{
		Dockerfile: "Dockerfile", // Path to your Dockerfile
		ContextDir: ".",          // Context directory for the Dockerfile
		Platform:   "linux/amd64",
		BuildArgs: []docker.BuildArg{
			{Name: "TARGETARCH", Value: targetArch},
		},
	}, &dockertest.RunOptions{
		Name:      "app",
		Env:       []string{fmt.Sprintf("DB_CONN_URL=%s", databaseUrl)},
		NetworkID: network.ID,
	}, func(config *docker.HostConfig) {
		config.NetworkMode = "bridge"
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	pool.MaxWait = 3 * time.Minute
	return appresource
}

func createMigration(err error, pool *dockertest.Pool, network *docker.Network, databaseUrl string, tempDir string, dbresource *dockertest.Resource) *dockertest.Resource {
	dbmigrate, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "migrate/migrate",
		Tag:        "latest",
		NetworkID:  network.ID,
		Cmd: []string{"-path", "/migrations",
			"-database", databaseUrl,
			"-verbose", "up", "2"},
		Mounts: []string{
			fmt.Sprintf("%s:/migrations", tempDir),
		},
	}, func(config *docker.HostConfig) {
		config.NetworkMode = "bridge"
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start dbresource: %s", err)
	}
	// Wait for the migration to complete
	if err := pool.Retry(func() error {
		_, err := dbmigrate.Exec([]string{"migrate", "-path", "/migrations", "-database", fmt.Sprintf("postgres://user_name:secret@localhost:%s/dbname?sslmode=disable", dbresource.GetPort("5432/tcp")), "up", "2"}, dockertest.ExecOptions{})
		return err
	}); err != nil {
		log.Fatalf("Migration failed: %s", err)
	}
	return dbmigrate
}

func createPostgresDB(err error, pool *dockertest.Pool, network *docker.Network) *dockertest.Resource {
	// pulls an image, creates a container based on it and runs it
	dbresource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "latest",
		Env: []string{
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_USER=user_name",
			"POSTGRES_DB=dbname",
			"listen_addresses = '*'",
		},
		NetworkID: network.ID,
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start dbresource: %s", err)
	}
	return dbresource
}

func findNetwork(networkName string, pool *dockertest.Pool) (*docker.Network, error) {
	networks, err := pool.Client.ListNetworks()
	if err != nil {
		return nil, err
	}
	for _, net := range networks {
		if net.Name == networkName {
			return &net, nil
		}
	}
	return nil, nil
}

func copyDir(src string, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, entry.Type()); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

func (l LocalTestContainer) Close() {
	err := l.dbcontainer.Close()
	if err != nil {
		log.Fatalf("Could not purge dbcontainer from test. Please delete manually.")
	}
	err = l.appcontainer.Close()
	if err != nil {
		log.Fatalf("Could not purge app container from test. Please delete manually.")
	}

	if err := l.pool.Client.RemoveNetwork(l.network); err != nil {
		log.Fatalf("Could not remove network: %s", err)
	}
}
