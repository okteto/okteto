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
	Space    string `json:"space" yaml:"space"`
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

//GetVolumeName returns the okteto volume name for a given database
func (db *DB) GetVolumeName() string {
	switch db.Name {
	case MONGO:
		return "mongo-persistent-storage"
	case REDIS:
		return "redis-persistent-storage"
	case POSTGRES:
		return "postgres-persistent-storage"
	}
	return ""
}

//GetVolumePath returns the okteto volume path for a given database
func (db *DB) GetVolumePath() string {
	switch db.Name {
	case MONGO:
		return "/data/db"
	case REDIS:
		return "/data"
	case POSTGRES:
		return "/var/lib/postgresql/data"
	}
	return ""
}
