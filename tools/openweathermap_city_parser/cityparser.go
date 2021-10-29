package main

import "fmt"
import "log"
import "strings"
import "io/ioutil"
import "encoding/json"
import "github.com/go-redis/redis"

const city_file = "city.list.json"
const redis_addr = "localhost:6379"
const redis_db = 0 // common db with settings shared between bots

type cityInfo struct {
    ID int64    `json:"id"`
    Name string `json:"name"`
    // there's something else but we don't need it
}

func main() {
    content, err := ioutil.ReadFile(city_file)
    if err != nil {
        log.Fatalf("Could not read contents of file '%s' due to error: %s", city_file, err)
    }
    cities := make([]cityInfo, 0, 250000)
    err = json.Unmarshal(content, &cities)
    if err != nil {
        log.Fatalf("Could not unmarshal json due to error: %s", err)
    }
    log.Printf("Parsed json contains %d records", len(cities))
    if len(cities) == 0 {
        panic("Zero cities")
    }

    log.Printf("Connecting to Redis (%s DB: %d)...", redis_addr, redis_db)
    opts := &redis.Options {
        Addr: redis_addr,
        DB: redis_db }
    conn := redis.NewClient(opts)
    log.Printf("Redis connected")
    for _, city := range cities {
        key := fmt.Sprintf("openweathermap:city:%s", strings.ToLower(city.Name))
        err = conn.HSet(key, "id", city.ID).Err()
        if err != nil {
            log.Printf("Could not store info about city %s (ID: %d)", city.Name, city.ID)
        }
    }
    log.Printf("File parsing and saving has been finished")
}
