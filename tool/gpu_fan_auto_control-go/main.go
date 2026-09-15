package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"

	"gpu_fan_auto_control/internal"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	if err := newRootCmd().Execute(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var port string

	root := &cobra.Command{
		Use:   "gpu_fan_auto_control",
		Short: "PWM fan control over serial",
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().StringVar(&port, "port", internal.DefaultSerialPort, "serial port")

	var setSpeed int
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set fan speed",
		RunE: func(cmd *cobra.Command, args []string) error {
			if setSpeed < 0 || setSpeed > internal.MaxFanSpeed {
				return fmt.Errorf("--speed must be 0-%d", internal.MaxFanSpeed)
			}
			sc := internal.NewSerialController(port)
			defer sc.Close()
			return sc.SetSpeedVerified(setSpeed)
		},
	}
	setCmd.Flags().IntVar(&setSpeed, "speed", -1, "fan speed 0-100")
	_ = setCmd.MarkFlagRequired("speed")

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get fan status",
		RunE: func(cmd *cobra.Command, args []string) error {
			sc := internal.NewSerialController(port)
			defer sc.Close()
			res, err := sc.GetSpeed(internal.StatusTimeout)
			if err != nil {
				return err
			}
			fmt.Println(res.Speed)
			return nil
		},
	}
	_ = getCmd.Flags().Bool("speed", false, "print current fan speed")
	_ = getCmd.MarkFlagRequired("speed")

	var payloadSpeed int
	payloadCmd := &cobra.Command{
		Use:   "payload",
		Short: "Print encoded cmd line (no serial)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if payloadSpeed < 0 || payloadSpeed > internal.MaxFanSpeed {
				return fmt.Errorf("--speed must be 0-%d", internal.MaxFanSpeed)
			}
			fmt.Println(internal.NewPayloadReq(payloadSpeed).Encode())
			return nil
		},
	}
	payloadCmd.Flags().IntVar(&payloadSpeed, "speed", -1, "fan speed 0-100")
	_ = payloadCmd.MarkFlagRequired("speed")

	var listen string
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "HTTP server for fan control",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(port, listen)
		},
	}
	serverCmd.Flags().StringVar(&listen, "listen", ":8080", "HTTP listen address")

	root.AddCommand(setCmd, getCmd, payloadCmd, serverCmd)
	return root
}

func registerFanMetrics(sc *internal.SerialController) {
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "gpu_fan_rpm",
			Help: "Current fan RPM",
		},
		func() float64 {
			if res := sc.Last(); res != nil {
				return float64(res.RPM)
			}
			return 0
		},
	))
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "gpu_fan_speed_percent",
			Help: "Current fan speed percent(0-100)",
		},
		func() float64 {
			if res := sc.Last(); res != nil {
				return float64(res.Speed)
			}
			return 0
		},
	))
}

func runServer(port, listen string) error {
	sc := internal.NewSerialController(port)
	defer sc.Close()
	if err := sc.EnsureReading(); err != nil {
		return err
	}

	registerFanMetrics(sc)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		res, err := sc.GetSpeed(internal.StatusTimeout)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{
			"speed": res.Speed,
			"rpm":   res.RPM,
		})
	})
	mux.HandleFunc("/speed/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/speed/"), "/")
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > internal.MaxFanSpeed {
			http.Error(w, fmt.Sprintf("speed must be 0-%d", internal.MaxFanSpeed), http.StatusBadRequest)
			return
		}
		if err := sc.SetSpeedVerified(n); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/speed", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		res, err := sc.GetSpeed(internal.StatusTimeout)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{
			"speed": res.Speed,
			"rpm":   res.RPM,
		})
	})

	log.Printf("listening on %s, serial %s", listen, port)
	return http.ListenAndServe(listen, mux)
}
