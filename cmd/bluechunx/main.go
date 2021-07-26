package main

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"tinygo.org/x/bluetooth"
)

type BxMessage struct {
	Addr string
	LName string
	Rssi int16
}
var adapter = bluetooth.DefaultAdapter

func valmaster(mofo map[string]string) []string {
	vals := make([]string, 0, len(mofo))
	for _,v := range mofo {
		vals = append(vals, v)
	}
	return vals
}

func main() {
	bx := make(map[string]string)
	bxAll := make(map[string]int)
	err := adapter.Enable()
	if err != nil {
		log.Error().Str("ugh", "ojsdf").Msg("adapter enable failed") 
	}
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	adapter.Enable()
	log.Info().Msg("Starting scan...")
    err = adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
		addr := device.Address.String()
		rssi := device.RSSI
		localname := device.LocalName()
		if _, ok := bxAll[addr]; !ok {
			bxAll[addr] = 1 
			if addr != "" {
				//rssiStr := strconv.FormatInt(int64(rssi), 10)
				m := BxMessage{addr, localname, rssi}
				//fmt.Printf("debu %v\n", m)
				jsonBytes, err := json.Marshal(m)
				if err != nil {
					log.Error().Str("err", "x").Msg("json marshal failed")
				}
				//log.Debug().Str("a", addr).Str("r", rssiStr).Str("n", localname).Msg(strconv.Itoa(len(bxAll)))
		    	log.Debug().RawJSON("j", jsonBytes).Msg(strconv.Itoa(len(bxAll)))
			}
		}
		if addr != "" && localname != "" {
			bx[addr] = localname
			jsonString, err := json.Marshal(valmaster(bx))
			if err != nil {
				log.Error().Str("err", "x").Msg("json marshal failed")
			}
		    log.Info().RawJSON("bx", jsonString).Msg(strconv.Itoa(len(bx)))
		}
	})
	if err != nil {
		log.Error().Str("ohno", "x").Msg("BOY")
	}

}
