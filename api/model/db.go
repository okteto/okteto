package model

import "fmt"

const (
	//REDIS the redis database type
	REDIS = "redis"
	//POSTGRES the postgres database type
	POSTGRES = "postgres"
	//MONGO the mongo database type
	MONGO = "mongo"
)

//DB represents a database
type DB struct {
	ID       string `json:"id" yaml:"id"`
	Name     string `json:"name" yaml:"name"`
	Password string
	Endpoint string `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
}

//GetEndpoint returns the dabatse endpoint
func (db *DB) GetEndpoint() string {
	switch db.Name {
	case MONGO:
		return "mongodb://mongo:27017"
	case REDIS:
		return "redis://redis:6379"
	case POSTGRES:
		return fmt.Sprintf("postgresql://okteto:%s@postgres:5432/db", db.Password)
	}
	return ""
}
