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

    "github.com/docopt/docopt-go"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"tinygo.org/x/bluetooth"
)

type BxMessage struct {
	//Addr string
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
    rdb *redis.Client
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
    usage := `blah
Usage:
  bluechunx [-h] [-r]

Options:
  -h --help    Show help info.
  -r           Redis mode.  Send scan results to redis.
    `
	bx := make(map[string]string)
	bxAll := make(map[string]int)

    arguments, _ := docopt.ParseDoc(usage)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
    mode_redis, _ := arguments.Bool("-r")
    log.Debug().Msgf("args: %v", mode_redis)
    //os.Exit(0)

	/*
	sampled := log.Sample(zerolog.LevelSampler{
    		DebugSampler: &zerolog.BurstSampler{
        		Burst: 1,
        		Period: 10*time.Second,
			// change this at will
			// you have the power
			// did the devs who wrote this even test it?
        		NextSampler: &zerolog.BasicSampler{N: 1000},
    	},
	})
	*/
	//sampled := log.Sample(&zerolog.BasicSampler{N: 10})


	hname, errH := os.Hostname()
	if errH != nil {
			log.Error().Err(errH).Msg("os.Hostname()")
	}

	log.Info().Msg("Starting prometheus http")
	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)
    http.Handle("/metrics", promhttp.Handler())
    srv := startHttpServer(httpServerExitDone)
    log.Info().Msgf("prometheus: %v", srv)

	err := adapter.Enable()
    //log.Info().Msgf("adapter id: %v", api.GetAdapterID())
	if err != nil {
		log.Error().Msg("adapter.Enable() failed")
	}

    if mode_redis {
	    rdb = RedisClient()
	    pong,_ := rdb.Ping(ctx).Result()
	    log.Info().Str("redisPing", pong).Msg("Redis server has ponged back.")
    }
    log.Info().Msg("-=[[ STARTING adapter.Scan() ]]=-")

    err = adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		addr := device.Address.String()
		rssi := device.RSSI
        localname := device.LocalName()
        now := time.Now()
		epoch := now.Unix()
		m := BxMessage{localname, rssi, epoch}
		jsonBytes, err := json.Marshal(m)
		if err != nil {
			log.Error().Str("err", "x").Msg("json marshal failed")
		}
		if _, ok := bxAll[addr]; !ok {
			bxAll[addr] = 1
			if addr != "" {


				//rssiStr := strconv.FormatInt(int64(rssi), 10)
				//log.Debug().Str("a", addr).Str("r", rssiStr).Str("n", localname).Msg(strconv.Itoa(len(bxAll)))


				addrsFound.Inc()
				if localname != "" {

                    // publish to redis pubsub and hash
                    if mode_redis {
                        _, errR := rdb.HMSet(ctx, "bluechunx:" + hname, addr, jsonBytes).Result()
				        if errR != nil {
					        log.Error().Str("err", "err").Msg("redis hmset failed")
				        }

	                    errRp := rdb.Publish(ctx, "bluechunx", jsonBytes).Err()
	                    if errRp != nil {
		                    log.Error().Str("err", "err").Msg("redis publish failed")
	                    }
                    }

					bx[addr] = localname
                    /*
                    names := valmaster(bx)
                    jsonBs, err := json.Marshal(names)
					if err != nil {
						log.Error().Str("err", "x").Msg("json marshal failed")
					}
                    */
					namedAddrsFound.Inc()
					//log.Info().RawJSON("bx", jsonBs).Msg(strconv.Itoa(len(bx)))
				    log.Info().RawJSON(addr, jsonBytes).Msg(strconv.Itoa(len(bxAll)))
					//log.Info().Str("n", "names found").Msg(strconv.Itoa(len(bx)))
				} else {
				    log.Debug().RawJSON(addr, jsonBytes).Msg(strconv.Itoa(len(bxAll)))
                }

			}
		}
	})
	if err != nil {
		log.Error().Msg("Scan startup failed")
	}

	if err := srv.Shutdown(context.TODO()); err != nil {
        panic(err) // failure/timeout shutting down the server gracefully
    }
    httpServerExitDone.Wait()
}
