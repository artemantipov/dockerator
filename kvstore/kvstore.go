package kvstore

import (
	"fmt"
	"log"
	"strings"

	"git.mills.io/prologic/bitcask"
)

// CountRS - count replicas for service
func CountRS(db *bitcask.Bitcask, name string) (rs int) {
	rs = 0
	rsInc := func(key []byte) error {
		rs++
		return nil
	}
	rsName := fmt.Sprintf("%v-", name)
	db.Scan([]byte(rsName), rsInc)
	return
}

// TasksList - return list of tasks
func TasksList(db *bitcask.Bitcask) (tasks []string) {
	tasks = []string{}
	appendTask := func(key []byte) error {
		tasks = append(tasks, string(key))
		return nil
	}
	db.Scan([]byte("Task"), appendTask)
	return
}

// KeyExist - check if key exist
func KeyExist(db *bitcask.Bitcask, key string) bool {
	return db.Has([]byte(key))
}

// InitDB - create db instance
func InitDB(path string) (db *bitcask.Bitcask) {
	opts := []bitcask.Option{
		bitcask.WithSync(true),
	}
	db, err := bitcask.Open(path, opts...)
	if err != nil {
		log.Fatal(err)
	}
	return
}

// PutKV - put k/v to config db
func PutKV(db *bitcask.Bitcask, key, value string) (err error) {
	err = db.Put([]byte(key), []byte(value))
	if err != nil {
		log.Printf("Error during inserting KV - %v", err)
	}
	return
}

// DeleteKV - delete key from config db
func DeleteKV(db *bitcask.Bitcask, key string) (result bool) {
	err := db.Delete([]byte(key))
	if err != nil {
		log.Printf("Error during deleting KV - %v", err)
		result = false
	} else {
		result = true
	}
	return
}

// GetKV - put k/v to config db
func GetKV(db *bitcask.Bitcask, key string) (value string, err error) {
	val, err := db.Get([]byte(key))
	value = string(val)
	return
}

// AppendKV - append value if key exist or create if not
func AppendKV(db *bitcask.Bitcask, key, value string) {
	if KeyExist(db, key) {
		v, _ := GetKV(db, key)
		if strings.Contains(v, value) != true {
			values := strings.Split(v, " ")
			value = strings.Join(append(values, value), " ")
			PutKV(db, key, value)
		}
	} else {
		PutKV(db, key, value)
	}
}

// EjectKV - exect on of values by key
func EjectKV(db *bitcask.Bitcask, key, value string) {
	val, _ := GetKV(db, key)
	values := strings.Split(val, " ")
	restValues := []string{}
	for _, v := range values {
		if v != value {
			restValues = append(restValues, v)
		}
	}
	resultValues := strings.Join(restValues, " ")
	PutKV(db, key, resultValues)
}
