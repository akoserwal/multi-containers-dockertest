package main

import (
	"database/sql"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"sync"
)

const defaultport = "3000"

type Item struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}

type GoPOS struct {
	db   *sql.DB
	port string
	host string
}

func main() {
	_ = flag.CommandLine.Parse([]string{})

	var rootCmd = &cobra.Command{
		Use:   "gopos",
		Short: "A simple golang app connects to postgresql.",
		Long:  `A simple golang app connects to postgresql`,
		Run:   serve,
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("error running command: %v", err)
	}

}

func newGpos(db *sql.DB, port string, host string) *GoPOS {
	return &GoPOS{
		db:   db,
		port: port,
		host: host,
	}
}

func (g GoPOS) runMigration() {

}

func serve(cmd *cobra.Command, args []string) {
	db, err := initDB()
	if err != nil {
		fmt.Println(err)
	}

	g := newGpos(db, "", "")
	router := gin.Default()
	router.GET("/health", g.getStatus)
	router.GET("/items", g.getItems)
	router.GET("/items/:id", g.getItem)
	router.POST("/items", g.createItem)
	router.PUT("/items/:id", g.updateItem)
	router.DELETE("/items/:id", g.deleteItem)

	wait := sync.WaitGroup{}
	go func() {
		err := router.Run(":8000")
		if err != nil {
			log.Println("Could not start http serving: ", err)
		}
	}()

	wait.Add(1)
	wait.Wait()
}

func (g *GoPOS) getItems(c *gin.Context) {
	rows, err := g.db.Query("SELECT id, name, price FROM items")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Price); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, items)
}

func (g *GoPOS) createItem(c *gin.Context) {
	var item Item
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := g.db.QueryRow("INSERT INTO items (name, price) VALUES ($1, $2) RETURNING id", item.Name, item.Price).Scan(&item.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (g *GoPOS) updateItem(c *gin.Context) {
	id := c.Param("id")
	var item Item
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := g.db.Exec("UPDATE items SET name = $1, price = $2 WHERE id = $3", item.Name, item.Price, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	c.JSON(http.StatusOK, item)
}

func (g *GoPOS) deleteItem(c *gin.Context) {
	id := c.Param("id")
	result, err := g.db.Exec("DELETE FROM items WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (g *GoPOS) getItem(c *gin.Context) {
	id := c.Param("id")
	var item Item
	err := g.db.QueryRow("SELECT id, name, price FROM items WHERE id = $1", id).Scan(&item.ID, &item.Name, &item.Price)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, item)
}

func (g GoPOS) Close() {
	g.db.Close()
}

func (g GoPOS) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func initDB() (*sql.DB, error) {
	viper.AutomaticEnv()

	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", 5432)

	user := viper.GetString("DB_USER")
	password := viper.GetString("DB_PASSWORD")
	dbname := viper.GetString("DB_NAME")
	dbhost := viper.GetString("DB_HOST")
	dbport := viper.GetInt("DB_PORT")
	dbconnurl := viper.GetString("DB_CONN_URL")

	_ = viper.GetString("GOPOS_HOST")
	port := viper.GetString("GOPOS_PORT")

	if port == "" {
		port = defaultport
	}

	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", user, password, dbhost, dbport, dbname)
	fmt.Println(connStr)

	if dbconnurl != "" {
		connStr = dbconnurl
		fmt.Println(connStr)
	}

	var err error
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Successfully connected to PostgreSQL!")
	return db, err
}
