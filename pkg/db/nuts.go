package db

import (
	"github.com/nutsdb/nutsdb"
	"log"
)

var (
	nutsDB *nutsdb.DB
)

func InitNutsDB(options nutsdb.Options, option ...nutsdb.Option) {
	var (
		err error
	)
	nutsDB, err = nutsdb.Open(
		// nutsdb.DefaultOptions,
		// TODO: support config
		// nutsdb.WithDir("./data/nutsdb"),
		options,
		option...,
	)
	if err != nil {
		log.Fatalln("open db error: ", err)
	}
}

func GetNutsDB() *nutsdb.DB {
	return nutsDB
}
