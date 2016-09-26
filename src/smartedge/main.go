/*
  The SmartEdge service allows any IoT device, cloud, sensor or similar to be easily integrated.

	Basic concepts, you need to create 4 exectuables that accept a JSON file as input:
	   (myservice)-in - command to get data from myservice
		 (myservice)-out - command to get my service to do an action or data
		 (myservice)-init - intialize the service - called 1 time
		 (myservice)-config - configure the service

		 The (myservice) can be wrapped inside an MQTT subscriber with the topic in/(myservice) and invoke (myservice)-in
		 or alternatively call smartedge inmqtt to get the JSON configuration to connect to the MQTT server to pass data.

		 The (myservice) can listen for actions on the MQTT topic out/(myservice). Just start the service with smartedge outmqtt.

		 To configure the initial service call:
		    smartedge config --dir "where config is stored" \
				    --transport "{Port: 1833, Host: localhost, ClientID: myservice, UserName: user, Password: pass, Certificate: ./cacert.crt}" \
						--init "{servicejson:toinit}" \
						--conf "{servicejson:to(re)conf}"
*/

package main

import (
  "gopkg.in/alecthomas/kingpin.v2"
	"log"
	"os"
	"os/exec"
  "os/signal"
	"time"
	"encoding/json"
	"bytes"
	"fmt"
	"github.com/yosssi/gmq/mqtt"
  "github.com/yosssi/gmq/mqtt/client"
	"crypto/x509"
	"crypto/tls"
  "io/ioutil"
)

var service = "notset"

type Transport struct {
    Port        string     // 1833 if not set
    Host        string  // Localhost if not set
		ClientID    string  // Mandatory
		Topic       string  // Mandatory
		UserName    string  // Optional
		Password    string  // Optional
		Certificate string  // Optional
}

// The In Interface receives data
type InItf struct {
		Data    string  // Mandatory - JSON format
}

// The Out Interface sends data and actions
type OutItf struct {
		Action    string  // Mandatory - JSON format
}

// The Config Interface does initial setup and configuration
type CnfItf struct {
   	Service       string  // Mandatory
		Transport     Transport // Optional
		InitService   string // Optional
		ConfigService string // Optional
}

// The Log interfac	d1 := []byte("hello\ngo\n")e shares internal logging information
type LogItf struct {
		Log      string  // Manadatory
		Err      bool  // Mandatory - default is true
}


var (
  se          = kingpin.New("smartedge", "A simple way to connect sensors, actuators, cloud and more on the edge.")
	in          = se.Command("in", "Get JSON data at specific time intervals and send it to the service")
	incfg       = in.Flag("config", "Config in JSON format the service needs").String()
  intime      = in.Flag("time", "How many times does the data get read (in seconds). Default is each second").Default("1").Int()
	inconfdir   = in.Flag("dir","In which directory is the configuration stored").Default(".").String()

	inmqtt      = se.Command("inmqtt", "Get MQTT config data in JSON format where data needs to be send.")
	inmconfdir  = inmqtt.Flag("dir","In which directory is the configuration stored").Default(".").String()

	outmqtt     = se.Command("outmqtt", "Start listening on MQTT for actions that need to be send.")
	outmconfdir = outmqtt.Flag("dir","In which directory is the configuration stored").Default(".").String()

	out         = se.Command("out", "Action the service needs to do")
	action      = out.Arg("action", "Action in JSON format to execute").Required().String()
	outconfdir  = out.Flag("dir","In which directory is the configuration stored").Default(".").String()

  logCmd      = se.Command("log", "Log data")
  logArg      = logCmd.Arg("log", "The data to log.").Required().String()
	errFlag     = logCmd.Flag("err", "Is this an error or a log entry.").Default("true").Bool()

  conf   = se.Command("config", "The transport the service needs to use")
	confdir     = conf.Flag("dir","In which directory is the configuration stored").Default(".").String()
	transjson   = conf.Flag("transport", "Setup the transport via a JSON file").String()
	initjson    = conf.Flag("init", "The JSON to initialize the service").String()
	configjson  = conf.Flag("conf", "The JSON to (re)configure the service").String()
	clientid    = conf.Flag("clientid", "The client id to use").String()
	host        = conf.Flag("host", "The hostname to connect to").Default("localhost").String()
	port        = conf.Flag("port", "The port number to connect to").Default("1833").String()
	user        = conf.Flag("user", "The user to use to connect").String()
	password    = conf.Flag("password", "The password to use to connect").String()
	certificate = conf.Flag("certificate", "The cerfiticate file").String()
)

func execute(cmd string, args []string) string {
	  var out bytes.Buffer
		cmdExec := exec.Command(cmd, args...)
    cmdExec.Stdout = &out
		if err := cmdExec.Run(); err != nil {
			errMsg(err.Error())
		}
		return out.String()
}

func executeSend(cmd string, args []string, cli client.Client) {

		sendIn(execute(cmd,args), cli)
}

func errMsg(logMsg string) {
	 fmt.Fprintf(os.Stderr, "%v\n", logMsg)
}
func logMsg(logMsg string) {
   fmt.Fprintf(os.Stdout, "%v\n", logMsg)
}

func readTransport(confdir string) *Transport {
	transfile, err := ioutil.ReadFile(confdir+"/transport.json")
	check(err)
	return readTransportJSON(transfile)
}

func readTransportJSON(trjson []byte) *Transport {
	var dat map[string]interface{}
	err := json.Unmarshal(trjson, &dat)
	check(err)
	reply := &Transport{
			Port:        dat["Port"].(string),
			Host:        dat["Host"].(string),
			ClientID:    dat["ClientID"].(string),
			UserName:    dat["UserName"].(string),
			Password:    dat["Password"].(string),
			Certificate: dat["Certificate"].(string),
		}
		service = dat["ClientID"].(string)
	return reply
}

func writeTransport(trans Transport, confdir string) {
	bytejson, _ := json.Marshal(trans)
  err := ioutil.WriteFile(confdir+"/transport.json", bytejson, 0600)
	check(err)
}

func sendIn(data string, cli client.Client) {
	// Publish a message.
    err := cli.Publish(&client.PublishOptions{
        QoS:       mqtt.QoS0,
        TopicName: []byte("in/"+service),
        Message:   []byte(data),
    })
    if err != nil {
        errMsg(err.Error())
    }

}

func waitForActions(transportCfg Transport, cli client.Client) {
	// Subscribe to topics.
  cli.Subscribe(&client.SubscribeOptions{
		SubReqs: []*client.SubReq{
				&client.SubReq{
						// TopicFilter is the Topic Filter of the Subscription.
						TopicFilter: []byte("out/"+service),
						// QoS is the requsting QoS.
						QoS: mqtt.QoS0,
						// Handler is the handler which handles the Application Message
						// sent from the Server.
						Handler: func(topicName, message []byte) {
								cmd := service+"-out"
								args := []string{string(message)}
								executeSend(cmd,args,cli)
						},
				},
		  },
  })
}

func connect(transportCfg Transport)  *client.Client {
	// Create an MQTT Client.
	cli := client.New(&client.Options{
			// Define the processing of the error handler.
			ErrorHandler: func(connecterr error) {
				 errMsg(connecterr.Error())
			},
	})

	// Terminate the Client.
	defer cli.Terminate()

  if transportCfg.Certificate != "" {
		roots := x509.NewCertPool()
		if ok := roots.AppendCertsFromPEM([]byte(transportCfg.Certificate)); !ok {
				errMsg("failed to parse root certificate")
		}
		tlsConfig := &tls.Config{
	    RootCAs: roots,
	  }

		err := cli.Connect(&client.ConnectOptions{
				Address:  transportCfg.Host+":"+transportCfg.Port,
				ClientID: []byte(transportCfg.ClientID),
				UserName: []byte(transportCfg.UserName),
				Password: []byte(transportCfg.Password),
				TLSConfig: tlsConfig,
		})
		if err != nil {
				errMsg(err.Error())
		}
	} else {

		err := cli.Connect(&client.ConnectOptions{
				Address:  transportCfg.Host+":"+transportCfg.Port,
				ClientID: []byte(transportCfg.ClientID),
				UserName: []byte(transportCfg.UserName),
				Password: []byte(transportCfg.Password),
		})
		if err != nil {
				errMsg(err.Error())
		}
	}

	return cli
}

func check(e error) {
    if e != nil {
        panic(e)
    }
}


func isInit(confdir string) {
	if _, err := os.Stat(confdir+"/transport.json"); os.IsNotExist(err) {
	 panic("The service was not initialized yet. Please call smartedge conf first to set up the service name and create the configuration files first.")
 }
}

func main() {

	switch kingpin.MustParse(se.Parse(os.Args[1:])) {
  // Receive data
  case in.FullCommand():
		isInit(*inconfdir)
		transportCfg := readTransport(*inconfdir)
		service = transportCfg.ClientID
		cmd := service+"-in"
		log.Print(cmd)
		args := []string{*incfg}
		// Connect to the MQTT Server.
    cli := connect(*transportCfg)
		for {
				go executeSend(cmd,args, *cli)
				time.Sleep(time.Duration(*intime) * time.Second)
		}

  // Tell the service where to send the data
  case inmqtt.FullCommand():
		isInit(*inmconfdir)
		intransport := readTransport(*inmconfdir)
		service = intransport.ClientID
		intransport.Topic = "in/"+service
    replyJSON, _ := json.Marshal(intransport)
		fmt.Fprintf(os.Stdout, "%v\n", string(replyJSON))

	// Send action
  case out.FullCommand():
		isInit(*outconfdir)
		outtransport := readTransport(*outconfdir)
		service = outtransport.ClientID
		cmd := service+".out"
		args := []string{*action}
    execute(cmd,args)

  // Wait for actions to arrive on mqtt
case outmqtt.FullCommand():
	  isInit(*outmconfdir)
	  // Set up channel on which to send signal notifications.
    sigc := make(chan os.Signal, 1)
    signal.Notify(sigc, os.Interrupt, os.Kill)
	  // Connect to the MQTT Server.
		outtransport := readTransport(*outmconfdir)
		service = outtransport.ClientID
		outtransport.Topic = "out/"+service
    cli := connect(*outtransport)
    waitForActions(*outtransport,*cli)
		// loop while waiting for commands to come in
		// Wait for receiving a signal.
    <-sigc

    // Disconnect the Network Connection.
    if err := cli.Disconnect(); err != nil {
        panic(err)
    }

	// Send log
  case logCmd.FullCommand():
		log.Print(*logArg)
    if *errFlag {
			errMsg(*logArg)
		} else {
			logMsg(*logArg)
		}


  case conf.FullCommand():
		if *transjson != "" {
        transport := readTransportJSON([]byte(*transjson))
				writeTransport(*transport, *confdir)
				service = transport.ClientID
		} else if *clientid != "" {
			if *certificate != "" {
				certificatedat, err := ioutil.ReadFile(*certificate)
				check(err)
				writeTransport(Transport{
						Port:        *port,
						Host:        *host,
						ClientID:    *clientid,
						UserName:    *user,
						Password:    *password,
						Certificate: string(certificatedat),
				 }, *confdir)
        service = *clientid
      } else {
					writeTransport(Transport{
							Port:        *port,
							Host:        *host,
							ClientID:    *clientid,
							UserName:    *user,
							Password:    *password,
				   }, *confdir)
				}
				service = *clientid
		} else {
			transport := readTransport(*confdir)
			service = transport.ClientID
		}



		if *initjson != "" {
			err := ioutil.WriteFile(*confdir+"/initservice.json", []byte(*initjson), 0600)
			check(err)
			logMsg(execute(service+"-init",[]string{*initjson}))
		}

		if *configjson != "" {
			err := ioutil.WriteFile(*confdir+"/configservice.json", []byte(*configjson), 0600)
			check(err)
			logMsg(execute(service+"-config",[]string{*configjson}))
		}
	}
}
