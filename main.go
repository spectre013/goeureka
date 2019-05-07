package fairway

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

//EurekaClient ...
type EurekaClient struct {
	Client Eureka
	Routes Routes
}

//EurekaConfig ...
type EurekaConfig struct {
	Name        string
	URL         string
	VipAddress  string
	IPAddress   string
	HostName    string
	Port        string
	SecurePort  string
	RestService bool
	PreferIP    bool
	Username    string
	Password    string
	Secure      bool
}

type secureStruct struct {
	User     string
	Password string
	Enable   bool
}

var logger = logrus.New()
var logLevel = logrus.InfoLevel
var secure = secureStruct{}
var appRoutes = Routes{}

// Init function for setting up Eureka Client
func Init(config EurekaConfig) EurekaClient {

	logger.Out = os.Stdout
	logger.SetLevel(logLevel)

	config.IPAddress = getOutboundIP().String()
	config.VipAddress = config.Name

	if config.PreferIP {
		config.HostName = config.IPAddress
	}
	secure.Enable = false

	if config.Secure {
		secure.User = config.Username
		secure.Password = config.Password
		secure.Enable = config.Secure
	}

	fmt.Println("Starting up ", config.Name)
	fmt.Println("########################################################")
	fmt.Println()
	fmt.Println()

	logger.Printf("%v", config)
	handleSigterm(config) // Graceful shutdown on Ctrl+C or kill
	go Register(config)   // Performs Eureka registration
	// start server and Block if not a rest service...
	if !config.RestService {
		go startWebServer(routes, config.Port)
		wg := sync.WaitGroup{} // Use a WaitGroup to block main() exit
		wg.Add(1)
		wg.Wait()
	}

	var e Eureka
	return EurekaClient{Client: e, Routes: routes}
}

func getOutboundIP() net.IP {

	interfaces, err := net.Interfaces()

	if err != nil {

		fmt.Print(err)
		os.Exit(0)
	}

	var result net.IP
	var loopback net.IP
	lowest := math.MaxInt64
	for _, i := range interfaces {
		addr, _ := i.Addrs()
		for _, a := range addr {
			logger.Debug("Interface: ", a.String())
			ip, err := getIP(a)
			if err != nil {
				fmt.Println("Error Getting IP ADDRESS")
			}
			ipv4 := ip.To4()
			if ipv4 != nil && !ipv4.IsLoopback() {
				logger.Debug("IPV4, up, isLoopback :", ipv4.String(), isUp(i.Flags.String()), ipv4.IsLoopback())
				if isUp(i.Flags.String()) {
					logger.Println(i.Index, lowest)
					if i.Index < lowest {
						result = ipv4
						lowest = i.Index
					}
				}
			} else {
				if ipv4 != nil {
					loopback = ipv4
				}
			}
		}
	}
	if result == nil {
		result = loopback
	}
	return result
}

func getIP(i net.Addr) (net.IP, error) {
	ip, _, err := net.ParseCIDR(i.String())
	if err != nil {
		return nil, err
	}
	return ip, nil
}

func isUp(flag string) bool {
	if strings.Contains(flag, "up") {
		return true
	}
	return false
}

func handleSigterm(config EurekaConfig) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP)
	go func() {
		<-c
		logger.Info(c)
		deregister(config)
		os.Exit(1)
	}()
}

// CombineRoutes Combines routes from external source with actuator routes
func CombineRoutes(routes Routes, eurekaRouts Routes) Routes {
	for _, route := range routes {
		eurekaRouts = append(eurekaRouts, route)
	}
	appRoutes = eurekaRouts
	return eurekaRouts
}

func startWebServer(routes Routes, port string) {
	router := mux.NewRouter()
	router = BuildRoutes(routes, router)
	logger.Info("Server is up and listening on ", port)
	http.ListenAndServe(port, router)
}
