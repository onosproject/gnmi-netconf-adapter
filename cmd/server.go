// Copyright 2020-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"net"
	"strconv"

	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	grpccredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	log "k8s.io/klog"
)

var (
	ca         *string
	key        *string
	cert       *string
	isInsecure *bool
	port       *int
	// The initial prototype only supports one device per adapter instance
	deviceIP       *string
	deviceUsername *string
	devicePassword *string
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Starts up a gNMI listener",
	Run:   RunGnmiServer,
}

func init() {

	// default paths to certs and key match the relative locations in docker image
	ca = serverCmd.Flags().String("ca", "certs/onfca.crt", "path to CA certificate")
	key = serverCmd.Flags().String("key", "certs/localhost.key", "path to client private key")
	cert = serverCmd.Flags().String("cert", "certs/localhost.crt", "path to client certificate")

	port = serverCmd.Flags().Int("port", 11161, "port to listen")
	isInsecure = serverCmd.Flags().Bool("insecure", false, "whether to enable skip verification")

	deviceIP = serverCmd.Flags().String("device-ip", "10.228.63.5:830", "device ip address:port for NETCONF")
	deviceUsername = serverCmd.Flags().String("device-user", "", "device NETCONF username")
	devicePassword = serverCmd.Flags().String("device-pass", "", "device NETCONF password")

	rootCmd.AddCommand(serverCmd)
}

func setupKlog() {
	// Refer https://github.com/onosproject/onos-config/issues/393
	//
	// https://github.com/kubernetes/klog/blob/master/examples/coexist_glog/coexist_glog.go
	// because of libraries importing glog. With glog import we can't call log.InitFlags(nil) as per klog readme
	// thus the alsologtostderr is not set properly and we issue multiple logs.
	// Calling log.InitFlags(nil) throws panic with error `flag redefined: log_dir`
	err := flag.Set("alsologtostderr", "true")
	if err != nil {
		log.Error("Cant' avoid double Error logging ", err)
	}
	flag.Parse()
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	log.InitFlags(klogFlags)
	// Sync the glog and klog flags.
	flag.CommandLine.VisitAll(func(f1 *flag.Flag) {
		f2 := klogFlags.Lookup(f1.Name)
		if f2 != nil {
			value := f1.Value.String()
			_ = f2.Value.Set(value)
		}
	})
}

// RunGnmiServer provides an indirection so that the logic can be tested independently of the cobra infrastructure
func RunGnmiServer(command *cobra.Command, args []string) {
	log.Info("Run GNMI Server... ")
	err := Serve(func(startedMsg string) {
		log.Info(startedMsg)
	})

	log.Exitf("Running Serve gave error=%v", err)

}

// Serve starts the NB gNMI server.
func Serve(started func(string)) error {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(*port))
	if err != nil {
		return err
	}

	tlsCfg := &tls.Config{}
	clientCerts, err := tls.LoadX509KeyPair(*cert, *key)
	if err != nil {
		return errors.Wrapf(err, "Couldn't load  X509 Key Pair using cert=%s and key=%s", *cert, *key)
	}
	tlsCfg.Certificates = []tls.Certificate{clientCerts}
	tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	tlsCfg.ClientCAs = getCertPool(*ca)
	if *isInsecure {
		tlsCfg.ClientAuth = tls.RequestClientCert
	} else {
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	opts := []grpc.ServerOption{grpc.Creds(grpccredentials.NewTLS(tlsCfg))}
	grpcServer := grpc.NewServer(opts...)

	s, err := newGnmiServer(model, *deviceIP, *deviceUsername, *devicePassword)
	if err != nil {
		return err
	}

	pb.RegisterGNMIServer(grpcServer, s)
	reflection.Register(grpcServer)

	message := fmt.Sprintf("Listening on %s with session opened to NETCONF device at %s", lis.Addr(), *deviceIP)
	started(message)
	return grpcServer.Serve(lis)
}

func getCertPool(CaPath string) *x509.CertPool {
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(CaPath)
	if err != nil {
		log.Warningf("could not read file at %s. Err is %v", CaPath, err)
	}
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		log.Warning("failed to append CA certificates")
	}
	return certPool
}
