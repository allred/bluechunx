package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"tinygo.org/x/bluetooth"
)

type BxMessage struct {
	Addr string
	LName string
	Rssi int16
	Epoch int64
}

var (
	ctx = context.Background()
	adapter = bluetooth.DefaultAdapter
	addrsFound = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bluechunx_addrs_found_total",
		Help: "Total addrs found during execution so far",
	})
	namedAddrsFound = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bluechunx_named_addrs_found_total",
		Help: "Total named addrs found during execution so far",
	})
)

func recordMetrics() {
	go func() {
		for {
			time.Sleep(2 * time.Second)
		}
	}()
}

func valmaster(mofo map[string]string) []string {
	vals := make([]string, 0, len(mofo))
	for _,v := range mofo {
		vals = append(vals, v)
	}
	return vals
}

func RedisClient() *redis.Client {
	opt, err := redis.ParseURL("redis://" + os.Getenv("BLUECHUNX_REDIS_URL"))
	if err != nil {
		fmt.Printf("redis parseurl fail ", err)
	}
	//rdb := redis.NewClient(&redis.Options{
	rdb := redis.NewClient(opt)
	return rdb
}

func startHttpServer(wg *sync.WaitGroup) *http.Server {
    srv := &http.Server{Addr: ":2112"}

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        io.WriteString(w, "hello world\n")
    })

    go func() {
        defer wg.Done() // let main know we are done cleaning up

        // always returns error. ErrServerClosed on graceful close
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Error().Err(err).Msg("ListenAndServe()")
        }
    }()

    // returning reference so caller can call Shutdown()
    return srv
}

func main() {
	bx := make(map[string]string)
	bxAll := make(map[string]int)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	hname, errH := os.Hostname()
	if errH != nil {
			log.Error().Err(errH).Msg("os.Hostname()")
	}

	log.Info().Msg("Starting prometheus http")
	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)
    http.Handle("/metrics", promhttp.Handler())
    srv := startHttpServer(httpServerExitDone)
	fmt.Printf("srv %v", srv)

	err := adapter.Enable()
	if err != nil {
		log.Error().Str("ugh", "ojsdf").Msg("adapter enable failed") 
	}

	rdb := RedisClient()
	pong, err := rdb.Ping(ctx).Result()
	log.Info().Str("redisPing", pong).Msg("Starting scan...")

    err = adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		addr := device.Address.String()
		rssi := device.RSSI
		localname := device.LocalName()
		if _, ok := bxAll[addr]; !ok {
			bxAll[addr] = 1 
			if addr != "" {
				//rssiStr := strconv.FormatInt(int64(rssi), 10)
				now := time.Now()
				epoch := now.Unix()
				m := BxMessage{addr, localname, rssi, epoch}
				jsonBytes, err := json.Marshal(m)
				if err != nil {
					log.Error().Str("err", "x").Msg("json marshal failed")
				}
				//log.Debug().Str("a", addr).Str("r", rssiStr).Str("n", localname).Msg(strconv.Itoa(len(bxAll)))

				_, errR := rdb.HMSet(ctx, "bluechunx:" + hname, addr, jsonBytes).Result()
				//fmt.Printf("debu %v\n", hash)
				if errR != nil {
					log.Error().Str("err", "err").Msg("redis hmset failed")
				}
				errRp := rdb.Publish(ctx, "bluechunx", jsonBytes).Err()
				if errRp != nil {
					log.Error().Str("err", "err").Msg("redis publish failed")
				}

				addrsFound.Inc()
		    	log.Debug().RawJSON("j", jsonBytes).Msg(strconv.Itoa(len(bxAll)))
			}
		}
		if addr != "" && localname != "" {
			bx[addr] = localname
			jsonString, err := json.Marshal(valmaster(bx))
			if err != nil {
				log.Error().Str("err", "x").Msg("json marshal failed")
			}
			namedAddrsFound.Inc()
		    log.Info().RawJSON("bx", jsonString).Msg(strconv.Itoa(len(bx)))
		}
	})
	if err != nil {
		log.Error().Str("ohno", "x").Msg("BOY")
	}

	if err := srv.Shutdown(context.TODO()); err != nil {
        panic(err) // failure/timeout shutting down the server gracefully
    }
    httpServerExitDone.Wait()
}
